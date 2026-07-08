package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	mitmproxy "github.com/lqqyt2423/go-mitmproxy/proxy"

	"shadowschema/internal/proxy"
	"shadowschema/internal/router"
	"shadowschema/internal/spec"
	wstap "shadowschema/internal/websocket"
)

var (
	targetDomain = flag.String("target", "example.com", "Target domain to intercept and map")
	port         = flag.String("port", ":38080", "Port to run the MITM proxy on")
	exportPort   = flag.String("export-port", "", "Port/addr for export server (default :38081 or SHADOWSCHEMA_EXPORT_ADDR)")
)

// Common auth-related request headers captured into the Auth Vault.
var vaultAuthHeaders = []string{
	"Authorization",
	"X-Api-Key",
	"X-Auth-Token",
	"Session-Token",
	"X-Access-Token",
	"X-CSRF-Token",
	"X-XSRF-Token",
	"Api-Key",
	"X-Api-Token",
	"X-Session-Token",
	"X-Token",
}

func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", port)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func requestHost(f *mitmproxy.Flow) string {
	if f == nil || f.Request == nil {
		return ""
	}
	host := f.Request.URL.Host
	if host == "" {
		host = f.Request.Header.Get("Host")
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

type ShadowSchemaAddon struct {
	mitmproxy.BaseAddon
	specManager *spec.SpecManager
}

// Request intercepts incoming requests
func (a *ShadowSchemaAddon) Request(f *mitmproxy.Flow) {
	if f.Request == nil {
		return
	}

	// Delete Accept-Encoding to prevent compressed bodies from target server
	f.Request.Header.Del("Accept-Encoding")

	host := requestHost(f)
	// Save credentials if present (scoped to request host)
	for _, h := range vaultAuthHeaders {
		if val := f.Request.Header.Get(h); val != "" {
			a.specManager.SaveVaultCredential(h, val, host)
		}
	}
	if cookie := f.Request.Header.Get("Cookie"); cookie != "" {
		a.specManager.SaveVaultCredential("Cookie", cookie, host)
	}

	if strings.ToLower(f.Request.Header.Get("Upgrade")) == "websocket" {
		dedupedPath := router.DeduplicatePath(f.Request.URL.Path)
		a.specManager.AddWebSocket(f.Request.Raw(), dedupedPath)
		fmt.Printf("[WS]   %-6s %s -> %s\n", f.Request.Method, f.Request.URL.Path, dedupedPath)
		return
	}

	// Log HTTP request
	fmt.Printf("[REQ]  %-6s %s\n", f.Request.Method, f.Request.URL.Path)
}

// Response intercepts responses
func (a *ShadowSchemaAddon) Response(f *mitmproxy.Flow) {
	if f.Response == nil || f.Request == nil {
		return
	}

	host := requestHost(f)
	// Capture Set-Cookie from responses into the vault (auth establishment).
	for _, c := range f.Response.Header.Values("Set-Cookie") {
		if c != "" {
			a.specManager.SaveVaultCredential("Set-Cookie", c, host)
		}
	}

	// Decode body if it is compressed
	bodyBytes, err := f.Response.DecodedBody()
	if err != nil {
		bodyBytes = f.Response.Body
	}

	reqBody := f.Request.Body
	if decoded, err := f.Request.DecodedBody(); err == nil && decoded != nil {
		reqBody = decoded
	}

	dedupedPath := router.DeduplicatePath(f.Request.URL.Path)
	fmt.Printf("[RESP] %-6d %s -> %s\n", f.Response.StatusCode, f.Request.URL.Path, dedupedPath)

	code := f.Response.StatusCode
	// Map successful responses (including empty 204) and error responses with bodies.
	// Skip informational (1xx) and redirects (3xx) to reduce noise.
	switch {
	case code >= 200 && code < 300:
		a.specManager.AddEndpoint(f.Request.Raw(), dedupedPath, code, bodyBytes, reqBody)
	case code >= 400 && code < 600 && len(bodyBytes) > 0:
		a.specManager.AddEndpoint(f.Request.Raw(), dedupedPath, code, bodyBytes, reqBody)
	}
}

// WebSocketStart hook
func (a *ShadowSchemaAddon) WebSocketStart(f *mitmproxy.Flow) {
	dedupedPath := router.DeduplicatePath(f.Request.URL.Path)
	a.specManager.AddWebSocket(f.Request.Raw(), dedupedPath)
	fmt.Printf("[WS]   %-6s %s -> %s\n", f.Request.Method, f.Request.URL.Path, dedupedPath)
}

// WebSocketMessage hook
func (a *ShadowSchemaAddon) WebSocketMessage(f *mitmproxy.Flow) {
	if f.WebScoket == nil || len(f.WebScoket.Messages) == 0 {
		return
	}
	msg := f.WebScoket.Messages[len(f.WebScoket.Messages)-1]

	dedupedPath := router.DeduplicatePath(f.Request.URL.Path)
	direction := "out"
	if msg.FromClient {
		direction = "in"
	}

	// WebSocket opcodes are a single byte (0–15 per RFC 6455); clamp before cast.
	opcode := byte(msg.Type & 0x0F)
	a.specManager.AddWebSocketFrame(dedupedPath, direction, opcode, msg.Content, 1)

	fmt.Printf("[WS]   %-3s  %s (%s, %d bytes)\n", strings.ToUpper(direction), dedupedPath, wstap.OpcodeName(opcode), len(msg.Content))
}

func certRootPath() string {
	if d := strings.TrimSpace(os.Getenv("SHADOWSCHEMA_CERT_DIR")); d != "" {
		return d
	}
	return "certs"
}

func newProxyServer(specManager *spec.SpecManager, port string) (*mitmproxy.Proxy, error) {
	opts := &mitmproxy.Options{
		Addr:        port,
		CaRootPath:  certRootPath(),
		SslInsecure: true,
	}

	p, err := mitmproxy.NewProxy(opts)
	if err != nil {
		return nil, err
	}

	p.SetShouldInterceptRule(func(req *http.Request) bool {
		host := req.URL.Host
		if host == "" {
			host = req.Host
		}
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}

		isTarget := specManager.IsTarget(host)
		if !isTarget {
			if req.Method == http.MethodConnect {
				specManager.AddDiscoveredDomain(host)
			}
			return false
		}
		return true
	})

	p.AddAddon(&ShadowSchemaAddon{
		specManager: specManager,
	})

	return p, nil
}

func resolveExportAddr() string {
	if *exportPort != "" {
		return *exportPort
	}
	if addr := strings.TrimSpace(os.Getenv("SHADOWSCHEMA_EXPORT_ADDR")); addr != "" {
		return addr
	}
	return ":38081"
}

func main() {
	flag.Parse()

	exportAddr := resolveExportAddr()

	if !isPortAvailable(*port) {
		log.Fatalf("Proxy port %s is already in use or unavailable\n", *port)
	}
	if !isPortAvailable(exportAddr) {
		log.Fatalf("Export address %s is already in use or unavailable\n", exportAddr)
	}

	// 1. Initialize CA
	if err := proxy.InitCA("certs"); err != nil {
		log.Fatalf("Failed to initialize CA: %v\n", err)
	}

	// 2. Initialize Spec Manager
	specManager := spec.NewSpecManager(*targetDomain)

	// Start export server in background
	go specManager.StartExportServer(exportAddr)

	p, err := newProxyServer(specManager, *port)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v\n", err)
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n[INFO] Gracefully shutting down...")
		specManager.Flush()
		if err := specManager.ExportJSON("openapi.json"); err != nil {
			fmt.Printf("[ERROR] Failed to export openapi.json: %v\n", err)
		} else {
			fmt.Println("[INFO] Successfully exported openapi.json")
		}
		_ = p.Close()
		os.Exit(0)
	}()

	fmt.Printf("Starting MITM API Mapper on %s (Default target: %s)\n", *port, *targetDomain)
	log.Fatal(p.Start())
}
