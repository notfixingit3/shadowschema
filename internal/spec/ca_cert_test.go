package spec

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"shadowschema/internal/proxy"
)

func TestCACertEndpointServesCertificate(t *testing.T) {
	setupIsolatedDB(t)
	if err := proxy.InitCA("certs"); err != nil {
		t.Fatalf("InitCA failed: %v", err)
	}

	sm := NewSpecManager("example.com")
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/ca-cert")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-pem-file" {
		t.Fatalf("unexpected content type: %s", ct)
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "shadowschema-ca.crt") {
		t.Fatalf("expected attachment filename, got %q", resp.Header.Get("Content-Disposition"))
	}

	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	if !strings.Contains(string(body[:n]), "BEGIN CERTIFICATE") {
		t.Fatalf("expected PEM certificate body")
	}
}

func TestCACertEndpointNotFoundWithoutCert(t *testing.T) {
	setupIsolatedDB(t)
	t.Setenv("SHADOWSCHEMA_CERT_DIR", "missing-certs-dir")

	sm := NewSpecManager("example.com")
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/ca-cert")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}