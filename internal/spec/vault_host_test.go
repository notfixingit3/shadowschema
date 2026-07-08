package spec

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVaultScopedByHost(t *testing.T) {
	sm := newTestSpecManager(t, "api.example.com,cdn.example.com")
	sm.SaveVaultCredential("Authorization", "Bearer api-token", "api.example.com")
	sm.SaveVaultCredential("Authorization", "Bearer cdn-token", "cdn.example.com")
	sm.SaveVaultCredential("X-Api-Key", "shared-global", "")

	apiCreds, err := sm.listVaultCredentialsForSession(sm.SessionID, "api.example.com")
	if err != nil {
		t.Fatal(err)
	}
	auth := ""
	key := ""
	for _, c := range apiCreds {
		if c.HeaderName == "Authorization" {
			auth = c.TokenValue
		}
		if c.HeaderName == "X-Api-Key" {
			key = c.TokenValue
		}
	}
	if auth != "Bearer api-token" {
		t.Fatalf("expected api host token, got %q among %#v", auth, apiCreds)
	}
	if key != "shared-global" {
		t.Fatalf("expected global key fallback, got %q", key)
	}

	cdnCreds, err := sm.listVaultCredentialsForSession(sm.SessionID, "cdn.example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cdnCreds {
		if c.HeaderName == "Authorization" && c.TokenValue != "Bearer cdn-token" {
			t.Fatalf("expected cdn token, got %q", c.TokenValue)
		}
	}

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/vault?include_values=1&host=api.example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var listed []AuthCredential
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range listed {
		if c.HeaderName == "Authorization" && c.TokenValue == "Bearer api-token" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected host-filtered vault response, got %#v", listed)
	}
}

// Parent-domain session targets (example.com) must still resolve vault tokens
// captured on API subdomains (api.example.com) — the common recon pattern.
func TestVaultFilterIncludesSubdomainCredentials(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	sm.SaveVaultCredential("Authorization", "Bearer api-sub-token", "api.example.com")
	sm.SaveVaultCredential("X-Api-Key", "parent-global", "")

	creds, err := sm.listVaultCredentialsForSession(sm.SessionID, "example.com")
	if err != nil {
		t.Fatal(err)
	}

	auth, key := "", ""
	for _, c := range creds {
		switch c.HeaderName {
		case "Authorization":
			auth = c.TokenValue
		case "X-Api-Key":
			key = c.TokenValue
		}
	}
	if auth != "Bearer api-sub-token" {
		t.Fatalf("expected subdomain token under parent filter, got %q among %#v", auth, creds)
	}
	if key != "parent-global" {
		t.Fatalf("expected global key under parent filter, got %q", key)
	}

	// More-specific filter still wins over sibling host.
	sm.SaveVaultCredential("Authorization", "Bearer exact", "example.com")
	exact, err := sm.listVaultCredentialsForSession(sm.SessionID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range exact {
		if c.HeaderName == "Authorization" && c.TokenValue != "Bearer exact" {
			t.Fatalf("exact host filter should prefer exact host credential, got %q", c.TokenValue)
		}
	}
}
