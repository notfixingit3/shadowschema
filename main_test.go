package main

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"shadowschema/internal/proxy"
	"shadowschema/internal/spec"
)

func TestIsPortAvailable(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	if !isPortAvailable(addr) {
		t.Fatalf("expected port %s to be available", addr)
	}

	ln2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("failed to bind reserved port: %v", err)
	}
	defer ln2.Close()

	if isPortAvailable(addr) {
		t.Fatalf("expected port %s to be unavailable while in use", addr)
	}
}

func TestProxyInterceptsTargetJSON(t *testing.T) {
	setupProxyTest(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"hello"}`))
	}))
	defer backend.Close()

	targetHost := hostFromURL(backend.URL)
	sm := spec.NewSpecManager(targetHost)
	proxyServer := httptest.NewServer(newProxyServer(sm))
	defer proxyServer.Close()

	client := proxiedClient(proxyServer.URL)
	resp, err := client.Get(backend.URL + "/api/hello")
	if err != nil {
		t.Fatalf("proxied request failed: %v", err)
	}
	_ = resp.Body.Close()

	assertExportHasPath(t, sm, "/api/hello")
}

func TestProxyCapturesVaultCredential(t *testing.T) {
	setupProxyTest(t)

	token := "Bearer main-test-" + t.Name()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	targetHost := hostFromURL(backend.URL)
	sm := spec.NewSpecManager(targetHost)
	proxyServer := httptest.NewServer(newProxyServer(sm))
	defer proxyServer.Close()

	req, err := http.NewRequest(http.MethodGet, backend.URL+"/api/secure", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", token)

	client := proxiedClient(proxyServer.URL)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("proxied request failed: %v", err)
	}
	_ = resp.Body.Close()

	exportServer := httptest.NewServer(sm.ExportHandler())
	defer exportServer.Close()

	vaultResp, err := http.Get(exportServer.URL + "/vault")
	if err != nil {
		t.Fatalf("vault request failed: %v", err)
	}
	defer vaultResp.Body.Close()

	var credentials []struct {
		HeaderName string `json:"header_name"`
		TokenValue string `json:"token_value"`
	}
	if err := json.NewDecoder(vaultResp.Body).Decode(&credentials); err != nil {
		t.Fatalf("failed to decode vault response: %v", err)
	}

	found := false
	for _, credential := range credentials {
		if credential.HeaderName == "Authorization" && credential.TokenValue == token {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected vault to include Authorization token %q, got %#v", token, credentials)
	}
}

func TestProxyRegistersWebSocketUpgrade(t *testing.T) {
	setupProxyTest(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	targetHost := hostFromURL(backend.URL)
	sm := spec.NewSpecManager(targetHost)
	proxyServer := httptest.NewServer(newProxyServer(sm))
	defer proxyServer.Close()

	req, err := http.NewRequest(http.MethodGet, backend.URL+"/ws/live", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")

	client := proxiedClient(proxyServer.URL)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("proxied websocket upgrade request failed: %v", err)
	}
	_ = resp.Body.Close()

	exported := fetchExportMap(t, sm)
	paths, ok := exported["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected paths in export, got %#v", exported["paths"])
	}

	pathItem, ok := paths["/ws/live"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected websocket path /ws/live, got %#v", paths)
	}

	getOp, ok := pathItem["get"].(map[string]interface{})
	if !ok || getOp["x-websocket"] != true {
		t.Fatalf("expected x-websocket operation, got %#v", pathItem)
	}
}

func TestProxyRecordsDiscoveredNonTargetHost(t *testing.T) {
	setupProxyTest(t)

	backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	targetHost := hostFromURL(backend.URL)
	sm := spec.NewSpecManager("not-" + targetHost)
	proxyServer := httptest.NewServer(newProxyServer(sm))
	defer proxyServer.Close()

	client := proxiedTLSClient(proxyServer.URL)
	resp, err := client.Get(backend.URL + "/api/other")
	if err != nil {
		t.Fatalf("proxied request failed: %v", err)
	}
	_ = resp.Body.Close()

	exportServer := httptest.NewServer(sm.ExportHandler())
	defer exportServer.Close()

	discoveredResp, err := http.Get(exportServer.URL + "/discovered")
	if err != nil {
		t.Fatalf("discovered request failed: %v", err)
	}
	defer discoveredResp.Body.Close()

	var discovered []string
	if err := json.NewDecoder(discoveredResp.Body).Decode(&discovered); err != nil {
		t.Fatalf("failed to decode discovered response: %v", err)
	}

	hostWithoutPort := strings.Split(targetHost, ":")[0]
	found := false
	for _, domain := range discovered {
		if strings.Contains(domain, hostWithoutPort) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected discovered domain containing %q, got %#v", hostWithoutPort, discovered)
	}
}

func setupProxyTest(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("NO_PROXY", "")

	certDir := filepath.Join(dir, "certs")
	if err := proxy.InitCA(certDir); err != nil {
		t.Fatalf("InitCA failed: %v", err)
	}
}

func hostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return parsed.Host
}

func proxiedClient(proxyURL string) *http.Client {
	return proxiedTLSClient(proxyURL)
}

func proxiedTLSClient(proxyURL string) *http.Client {
	proxyParsed, err := url.Parse(proxyURL)
	if err != nil {
		panic(err)
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyURL(proxyParsed),
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}
}

func fetchExportMap(t *testing.T, sm *spec.SpecManager) map[string]interface{} {
	t.Helper()

	exportServer := httptest.NewServer(sm.ExportHandler())
	defer exportServer.Close()

	resp, err := http.Get(exportServer.URL + "/export-map")
	if err != nil {
		t.Fatalf("export-map request failed: %v", err)
	}
	defer resp.Body.Close()

	var exported map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&exported); err != nil {
		t.Fatalf("failed to decode export-map: %v", err)
	}
	return exported
}

func assertExportHasPath(t *testing.T, sm *spec.SpecManager, path string) {
	t.Helper()

	exported := fetchExportMap(t, sm)
	paths, ok := exported["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected paths in export, got %#v", exported["paths"])
	}
	if _, ok := paths[path]; !ok {
		t.Fatalf("expected path %q in export, got %#v", path, paths)
	}
}