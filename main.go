package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/elazarl/goproxy"

	"shadowschema/internal/proxy"
	"shadowschema/internal/router"
	"shadowschema/internal/spec"
	wstap "shadowschema/internal/websocket"
)

var (
	targetDomain = flag.String("target", "example.com", "Target domain to intercept and map")
	port         = flag.String("port", ":38080", "Port to run the MITM proxy on")
	exportPort   = flag.String("export-port", ":38081", "Port to run the export server on")
)

func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", port)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func newProxyServer(specManager *spec.SpecManager) *goproxy.ProxyHttpServer {
	p := goproxy.NewProxyHttpServer()
	p.Verbose = false

	p.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if specManager.IsTarget(host) {
			return goproxy.MitmConnect, host
		}
		specManager.AddDiscoveredDomain(host)
		return goproxy.OkConnect, host
	})

	condition := goproxy.ReqConditionFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return specManager.IsTarget(req.URL.Host) || specManager.IsTarget(req.Host)
	})

	p.OnRequest(condition).DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			r.Header.Del("Accept-Encoding")

			authHeaders := []string{"Authorization", "X-Api-Key", "X-Auth-Token", "Session-Token"}
			for _, h := range authHeaders {
				if val := r.Header.Get(h); val != "" {
					specManager.SaveVaultCredential(h, val)
				}
			}

			if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
				dedupedPath := router.DeduplicatePath(r.URL.Path)
				specManager.AddWebSocket(r, dedupedPath)
				fmt.Printf("[WS]   %-6s %s -> %s\n", r.Method, r.URL.Path, dedupedPath)
				return r, nil
			}

			fmt.Printf("[REQ]  %-6s %s\n", r.Method, r.URL.Path)
			return r, nil
		})

	p.OnResponse(condition).DoFunc(
		func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			if resp == nil || resp.Body == nil {
				return resp
			}

			if wstap.IsUpgradeResponse(resp) && ctx.Req != nil {
				dedupedPath := router.DeduplicatePath(ctx.Req.URL.Path)
				if rw, ok := resp.Body.(io.ReadWriter); ok {
					resp.Body = wstap.NewFrameTap(rw, func(direction string, opcode byte, payload []byte, info wstap.FrameInfo) {
						specManager.AddWebSocketFrame(dedupedPath, direction, opcode, payload, info.Fragments)
						fragNote := ""
						if info.Fragments > 1 {
							fragNote = fmt.Sprintf(", %d frags", info.Fragments)
						}
						fmt.Printf("[WS]   %-3s  %s (%s, %d bytes%s)\n", strings.ToUpper(direction), dedupedPath, wstap.OpcodeName(opcode), len(payload), fragNote)
					})
				}
				return resp
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err == nil {
				resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				dedupedPath := router.DeduplicatePath(ctx.Req.URL.Path)
				fmt.Printf("[RESP] %-6d %s -> %s\n", resp.StatusCode, ctx.Req.URL.Path, dedupedPath)

				if resp.StatusCode >= 200 && resp.StatusCode < 300 && len(bodyBytes) > 0 {
					specManager.AddEndpoint(ctx.Req, dedupedPath, bodyBytes)
				}
			}
			return resp
		})

	return p
}

func main() {
	flag.Parse()

	if !isPortAvailable(*port) {
		log.Fatalf("Proxy port %s is already in use or unavailable\n", *port)
	}
	if !isPortAvailable(*exportPort) {
		log.Fatalf("Export port %s is already in use or unavailable\n", *exportPort)
	}

	// 1. Initialize CA
	if err := proxy.InitCA("certs"); err != nil {
		log.Fatalf("Failed to initialize CA: %v\n", err)
	}

	// 2. Initialize Spec Manager
	specManager := spec.NewSpecManager(*targetDomain)

	// Start export server in background
	go specManager.StartExportServer(*exportPort)

	p := newProxyServer(specManager)

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n[INFO] Gracefully shutting down...")
		if err := specManager.ExportJSON("openapi.json"); err != nil {
			fmt.Printf("[ERROR] Failed to export openapi.json: %v\n", err)
		} else {
			fmt.Println("[INFO] Successfully exported openapi.json")
		}
		os.Exit(0)
	}()

	fmt.Printf("Starting MITM API Mapper on %s (Default target: %s)\n", *port, *targetDomain)
	srv := &http.Server{
		Addr:              *port,
		Handler:           p,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
