package spec

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExportMapEndpointIncludesVault(t *testing.T) {
	sm := NewSpecManager("example.com")
	sm.SaveVaultCredential("Authorization", "Bearer test-token")

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/export-map")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var exported map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&exported); err != nil {
		t.Fatalf("failed to decode export: %v", err)
	}

	vault, ok := exported["x-shadowschema-vault"].([]interface{})
	if !ok || len(vault) == 0 {
		t.Fatalf("expected vault data in export-map response, got %#v", exported["x-shadowschema-vault"])
	}
}

func TestVaultEndpointReturnsCredentials(t *testing.T) {
	sm := NewSpecManager("example.com")
	token := "secret-key-" + t.Name()
	sm.SaveVaultCredential("X-Api-Key", token)

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/vault")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var credentials []AuthCredential
	if err := json.NewDecoder(resp.Body).Decode(&credentials); err != nil {
		t.Fatalf("failed to decode vault response: %v", err)
	}

	found := false
	for _, credential := range credentials {
		if credential.HeaderName == "X-Api-Key" && credential.TokenValue == token {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected saved credential in vault payload, got %#v", credentials)
	}
}

func TestGenerateSDKEndpointRejectsUnsupportedLanguage(t *testing.T) {
	sm := NewSpecManager("example.com")

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/generate-sdk", "application/json", bytes.NewBufferString(`{"language":"ruby"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported language, got %d", resp.StatusCode)
	}
}