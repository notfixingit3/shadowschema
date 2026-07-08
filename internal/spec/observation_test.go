package spec

import (
	"net/http"
	"testing"
)

func TestObservationMultiSampleAndRequiredParams(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	// Always-present limit query param + occasional debug header.
	for i := 0; i < 4; i++ {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/items?limit=10", nil)
		if i%2 == 0 {
			req.Header.Set("X-Debug", "1")
		}
		sm.AddEndpoint(req, "/api/items", 200, []byte(`{"items":[]}`), nil)
	}

	pathItem := sm.doc.Paths.Find("/api/items")
	if pathItem == nil || pathItem.Get == nil {
		t.Fatal("expected GET /api/items")
	}
	op := pathItem.Get

	hits, ok := op.Extensions["x-hit-count"].(int)
	if !ok || hits != 4 {
		// JSON round-trip may use float64 only after marshal; in-memory should be int.
		if f, ok := op.Extensions["x-hit-count"].(float64); ok {
			hits = int(f)
		}
		if hits != 4 {
			t.Fatalf("expected hit count 4, got %#v", op.Extensions["x-hit-count"])
		}
	}

	samples, ok := op.Extensions["x-payload-samples"].([]interface{})
	if !ok || len(samples) == 0 {
		t.Fatalf("expected payload samples, got %#v", op.Extensions["x-payload-samples"])
	}

	var limitRequired bool
	var debugRequired bool
	for _, p := range op.Parameters {
		if p.Value == nil {
			continue
		}
		if p.Value.Name == "limit" && p.Value.In == "query" {
			limitRequired = p.Value.Required
			if p.Value.Schema == nil || p.Value.Schema.Value == nil || !p.Value.Schema.Value.Type.Is("integer") {
				t.Fatalf("expected integer type for limit, got %#v", p.Value.Schema)
			}
		}
		if p.Value.Name == "X-Debug" && p.Value.In == "header" {
			debugRequired = p.Value.Required
		}
	}
	if !limitRequired {
		t.Fatal("expected limit query param to be required after consistent observation")
	}
	if debugRequired {
		t.Fatal("expected intermittent X-Debug header not required")
	}
}

func TestSanitizeDocForExportStripsSecrets(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	req, _ := http.NewRequest(http.MethodPost, "http://example.com/api/items", nil)
	sm.AddEndpoint(req, "/api/items", 200, []byte(`{"secret":"x"}`), []byte(`{"password":"y"}`))
	sm.SaveVaultCredential("Authorization", "Bearer secret", "example.com")

	sm.mu.Lock()
	sdkDoc, _, err := specForSDK(sm.doc)
	sm.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if sdkDoc.Extensions != nil {
		if _, ok := sdkDoc.Extensions["x-shadowschema-vault"]; ok {
			t.Fatal("vault must not be in SDK spec")
		}
	}
	op := sdkDoc.Paths.Find("/api/items").Post
	if op.Extensions["x-last-payload"] != nil || op.Extensions["x-last-request-body"] != nil {
		t.Fatalf("sample payloads must be stripped for SDK, got %#v", op.Extensions)
	}
	if op.Extensions["x-payload-samples"] != nil {
		t.Fatal("payload samples must be stripped for SDK")
	}
}
