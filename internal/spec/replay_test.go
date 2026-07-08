package spec

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExportReplayEndpointReturnsPythonScript(t *testing.T) {
	sm := newTestSpecManager(t, "api.example.com")
	req, _ := http.NewRequest(http.MethodPost, "http://api.example.com/api/items", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	// response body vs request body must stay distinct for replay
	sm.AddEndpoint(req, "/api/items", 201, []byte(`{"id":42,"created":true}`), []byte(`{"name":"widget"}`))
	sm.SaveVaultCredential("Authorization", "Bearer test-token")

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/export-replay?path=/api/items&method=POST")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	script := string(body)
	if !strings.Contains(script, "import requests") {
		t.Fatalf("expected python script, got %q", script)
	}
	if !strings.Contains(script, "/api/items") {
		t.Fatalf("expected path in script")
	}
	if !strings.Contains(script, "payload =") {
		t.Fatalf("expected payload block for POST")
	}
	if !strings.Contains(script, "widget") {
		t.Fatalf("expected request body fields in payload, got %q", script)
	}
	if strings.Contains(script, `"created"`) {
		t.Fatalf("replay must not use response body as request payload: %q", script)
	}
}

func TestExportReplayWithoutRequestBody(t *testing.T) {
	sm := newTestSpecManager(t, "api.example.com")
	req, _ := http.NewRequest(http.MethodPost, "http://api.example.com/api/items", nil)
	sm.AddEndpoint(req, "/api/items", 200, []byte(`{"id":1}`), nil)

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/export-replay?path=/api/items&method=POST")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	script := string(body)
	if !strings.Contains(script, "No request body captured") {
		t.Fatalf("expected no-request-body comment, got %q", script)
	}
	if strings.Contains(script, "payload =") {
		t.Fatalf("should not invent payload from response: %q", script)
	}
}
