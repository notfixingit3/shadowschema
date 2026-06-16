package spec

import (
	"net/http"
	"testing"
)

func TestSpecManagerAddEndpoint(t *testing.T) {
	sm := NewSpecManager()

	req1, _ := http.NewRequest(http.MethodGet, "http://example.com/api/users", nil)
	sm.AddEndpoint(req1, "/api/users", []byte(`{"id": 1, "name": "Alice"}`))

	pathItem := sm.doc.Paths.Find("/api/users")
	if pathItem == nil {
		t.Fatalf("Expected path /api/users to be added")
	}

	if pathItem.Get == nil {
		t.Fatalf("Expected GET operation for /api/users")
	}

	// Add same endpoint with new fields
	req2, _ := http.NewRequest(http.MethodGet, "http://example.com/api/users?limit=10", nil)
	req2.Header.Set("X-Custom-Auth", "secret")
	sm.AddEndpoint(req2, "/api/users", []byte(`{"id": 2, "name": "Bob", "email": "bob@example.com"}`))

	resp := pathItem.Get.Responses.Value("200")
	if resp == nil {
		t.Fatalf("Expected 200 response")
	}

	schema := resp.Value.Content.Get("application/json").Schema
	if schema == nil {
		t.Fatalf("Expected JSON schema in response")
	}

	if _, ok := schema.Value.Properties["email"]; !ok {
		t.Errorf("Expected 'email' property to be merged into schema")
	}
}
