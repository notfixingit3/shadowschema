package spec

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestMigrateLegacyWebSocketSpecs(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.0",
		Paths:   openapi3.NewPaths(),
	}

	pathItem := &openapi3.PathItem{
		Trace: &openapi3.Operation{
			Summary:     "WebSocket Connection",
			Description: "Detected WebSocket upgrade on this endpoint.",
		},
	}
	doc.Paths.Set("/ws/chat", pathItem)

	if !migrateLegacyWebSocketSpecs(doc) {
		t.Fatalf("expected migration to modify the spec")
	}

	updated := doc.Paths.Find("/ws/chat")
	if updated == nil || updated.Get == nil {
		t.Fatalf("expected migrated GET operation")
	}
	if updated.Trace != nil {
		t.Fatalf("expected legacy TRACE operation to be removed")
	}
	if updated.Get.Extensions["x-websocket"] != true {
		t.Fatalf("expected x-websocket extension on migrated operation")
	}
}

func TestSpecForSDKExcludesWebSocketOperations(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.0",
		Paths:   openapi3.NewPaths(),
	}

	rest := &openapi3.PathItem{Get: openapi3.NewOperation()}
	doc.Paths.Set("/api/users", rest)

	ws := &openapi3.PathItem{Get: openapi3.NewOperation()}
	ws.Get.Extensions = map[string]interface{}{"x-websocket": true}
	doc.Paths.Set("/ws/chat", ws)

	filtered, excluded, err := specForSDK(doc)
	if err != nil {
		t.Fatalf("specForSDK failed: %v", err)
	}
	if excluded != 1 {
		t.Fatalf("expected 1 excluded websocket operation, got %d", excluded)
	}
	if filtered.Paths.Find("/api/users") == nil {
		t.Fatalf("expected REST path to remain")
	}
	if filtered.Paths.Find("/ws/chat") != nil {
		t.Fatalf("expected websocket-only path to be removed")
	}
}