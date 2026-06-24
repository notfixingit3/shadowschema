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
	sm.AddEndpoint(req, "/api/items", []byte(`{"id":42}`))
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
}