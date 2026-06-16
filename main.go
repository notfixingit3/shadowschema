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
	"regexp"
	"syscall"

	"github.com/elazarl/goproxy"

	"shadowschema/internal/proxy"
	"shadowschema/internal/router"
	"shadowschema/internal/spec"
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
	ln.Close()
	return true
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
	specManager := spec.NewSpecManager()

	// Start export server in background
	go specManager.StartExportServer(*exportPort)

	// 3. Initialize the proxy
	p := goproxy.NewProxyHttpServer()
	p.Verbose = false // Keep false to maintain clean terminal alignment

	// Ensure we MITM all HTTPS requests
	p.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	// Filter traffic: Only intercept the target domain
	condition := goproxy.ReqHostMatches(regexp.MustCompile(fmt.Sprintf(`.*%s.*`, regexp.QuoteMeta(*targetDomain))))

	// Handle Requests
	p.OnRequest(condition).DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			// Clean terminal output - Method and Path, aligned
			fmt.Printf("[REQ]  %-6s %s\n", r.Method, r.URL.Path)
			return r, nil
		})

	// Handle Responses
	p.OnResponse(condition).DoFunc(
		func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			if resp == nil || resp.Body == nil {
				return resp
			}

			// Read body for schema parsing
			bodyBytes, err := io.ReadAll(resp.Body)
			if err == nil {
				// Reassign body so the client still receives it
				resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				dedupedPath := router.DeduplicatePath(ctx.Req.URL.Path)

				// Clean terminal output - Status code, aligned
				fmt.Printf("[RESP] %-6d %s -> %s\n", resp.StatusCode, ctx.Req.URL.Path, dedupedPath)

				// Send ctx.Req to spec manager
				if resp.StatusCode >= 200 && resp.StatusCode < 300 && len(bodyBytes) > 0 {
					specManager.AddEndpoint(ctx.Req, dedupedPath, bodyBytes)
				}
			}
			return resp
		})

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

	fmt.Printf("Starting MITM API Mapper on %s targeting %s...\n", *port, *targetDomain)
	log.Fatal(http.ListenAndServe(*port, p))
}
