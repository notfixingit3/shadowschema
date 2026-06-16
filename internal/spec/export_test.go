package spec

import (
	"encoding/json"
	"testing"
)

func TestBuildExportDocumentIncludesVault(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	sm.SaveVaultCredential("Authorization", "Bearer test-token")
	sm.SaveVaultCredential("X-Api-Key", "secret-key")

	sm.mu.Lock()
	data, err := sm.buildExportDocument()
	sm.mu.Unlock()

	if err != nil {
		t.Fatalf("buildExportDocument failed: %v", err)
	}

	var exported map[string]interface{}
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("failed to parse export: %v", err)
	}

	vault, ok := exported["x-shadowschema-vault"].([]interface{})
	if !ok || len(vault) != 2 {
		t.Fatalf("expected vault extension with 2 credentials, got %#v", exported["x-shadowschema-vault"])
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