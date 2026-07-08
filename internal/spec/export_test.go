package spec

import (
	"encoding/json"
	"testing"
)

func TestBuildExportDocumentOmitsVaultSecretsByDefault(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	sm.SaveVaultCredential("Authorization", "Bearer test-token", "example.com")
	sm.SaveVaultCredential("X-Api-Key", "secret-key", "example.com")

	data, err := sm.buildExportDocument()

	if err != nil {
		t.Fatalf("buildExportDocument failed: %v", err)
	}

	var exported map[string]interface{}
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("failed to parse export: %v", err)
	}

	if _, ok := exported["x-shadowschema-vault"]; ok {
		t.Fatalf("default export must not embed vault secrets, got %#v", exported["x-shadowschema-vault"])
	}

	components, ok := exported["components"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected components in export")
	}
	schemes, ok := components["securitySchemes"].(map[string]interface{})
	if !ok || len(schemes) < 2 {
		t.Fatalf("expected security schemes from vault, got %#v", components["securitySchemes"])
	}
}

func TestBuildExportDocumentIncludesSecretsWhenRequested(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	sm.SaveVaultCredential("Authorization", "Bearer test-token", "example.com")

	sm.mu.Lock()
	data, err := sm.buildExportDocumentFrom(sm.doc, sm.SessionID, true)
	sm.mu.Unlock()
	if err != nil {
		t.Fatalf("buildExportDocumentFrom failed: %v", err)
	}

	var exported map[string]interface{}
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("failed to parse export: %v", err)
	}

	vault, ok := exported["x-shadowschema-vault"].([]interface{})
	if !ok || len(vault) == 0 {
		t.Fatalf("expected vault secrets when includeSecrets=true, got %#v", exported["x-shadowschema-vault"])
	}
}
