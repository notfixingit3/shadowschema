package spec

import (
	"net/http"
	"testing"
)

func TestSpecManagerAddEndpoint(t *testing.T) {
	sm := NewSpecManager()

	sm.AddEndpoint(http.MethodGet, "/api/users", []byte(`{"id": 1, "name": "Alice"}`))

	pathItem := sm.doc.Paths.Find("/api/users")
	if pathItem == nil {
		t.Fatalf("Expected path /api/users to be added")
	}

	if pathItem.Get == nil {
		t.Fatalf("Expected GET operation for /api/users")
	}

	// Add same endpoint with new fields
	sm.AddEndpoint(http.MethodGet, "/api/users", []byte(`{"id": 2, "name": "Bob", "email": "bob@example.com"}`))

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
