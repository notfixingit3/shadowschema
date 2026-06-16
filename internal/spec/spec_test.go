package spec

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSpecManagerAddEndpoint(t *testing.T) {
	sm := NewSpecManager("example.com")

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

func TestSpecManagerAddWebSocket(t *testing.T) {
	sm := NewSpecManager("example.com")

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/ws/chat/550e8400-e29b-41d4-a716-446655440000?token=abc123", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Protocol", "json")

	sm.AddWebSocket(req, "/ws/chat/{uuid}")

	pathItem := sm.doc.Paths.Find("/ws/chat/{uuid}")
	if pathItem == nil {
		t.Fatalf("Expected deduplicated path /ws/chat/{uuid} to be added")
	}

	if pathItem.Get == nil {
		t.Fatalf("Expected GET operation for websocket endpoint")
	}

	if pathItem.Trace != nil {
		t.Fatalf("Expected websocket to use GET with x-websocket, not TRACE")
	}

	if pathItem.Get.Extensions["x-websocket"] != true {
		t.Fatalf("Expected x-websocket extension to be set")
	}

	paramNames := make(map[string]string)
	for _, p := range pathItem.Get.Parameters {
		if p.Value != nil {
			paramNames[p.Value.Name] = p.Value.In
		}
	}

	if paramNames["token"] != "query" {
		t.Errorf("Expected query param 'token', got %v", paramNames)
	}
	if paramNames["Sec-Websocket-Key"] != "header" && paramNames["Sec-WebSocket-Key"] != "header" {
		t.Errorf("Expected Sec-WebSocket-Key header param, got %v", paramNames)
	}
	if paramNames["Sec-Websocket-Version"] != "header" && paramNames["Sec-WebSocket-Version"] != "header" {
		t.Errorf("Expected Sec-WebSocket-Version header param, got %v", paramNames)
	}
}

func TestSpecManagerAddWebSocketFrame(t *testing.T) {
	sm := NewSpecManager("example.com")
	path := fmt.Sprintf("/ws/frame-capture-%d", time.Now().UnixNano())

	req, _ := http.NewRequest(http.MethodGet, "http://example.com"+path, nil)
	sm.AddWebSocket(req, path)

	sm.AddWebSocketFrame(path, "in", 0x1, []byte(`{"event":"welcome","user_id":1}`), 1)
	sm.AddWebSocketFrame(path, "out", 0x1, []byte(`{"event":"subscribe","channel":"alerts"}`), 2)
	sm.AddWebSocketFrame(path, "in", 0x9, nil, 1)

	pathItem := sm.doc.Paths.Find(path)
	rawFrames, ok := pathItem.Get.Extensions["x-websocket-frames"].([]interface{})
	if !ok || len(rawFrames) != 3 {
		t.Fatalf("expected 3 captured websocket frames, got %#v", pathItem.Get.Extensions["x-websocket-frames"])
	}

	inSchema, ok := pathItem.Get.Extensions["x-websocket-message-schema-in"].(*openapi3.Schema)
	if !ok || inSchema == nil {
		t.Fatalf("expected inbound websocket schema, got %#v", pathItem.Get.Extensions["x-websocket-message-schema-in"])
	}
	outSchema, ok := pathItem.Get.Extensions["x-websocket-message-schema-out"].(*openapi3.Schema)
	if !ok || outSchema == nil {
		t.Fatalf("expected outbound websocket schema, got %#v", pathItem.Get.Extensions["x-websocket-message-schema-out"])
	}

	if _, ok := inSchema.Properties["event"]; !ok {
		t.Fatalf("expected inbound schema to include welcome event fields, got %#v", inSchema.Properties)
	}
	if _, ok := outSchema.Properties["channel"]; !ok {
		t.Fatalf("expected outbound schema to include subscribe channel fields, got %#v", outSchema.Properties)
	}

	stats, ok := pathItem.Get.Extensions["x-websocket-stats"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected websocket stats")
	}
	if websocketStatValue(stats, "total") != 3 {
		t.Fatalf("expected 3 total frames in stats, got %#v", stats)
	}
	if websocketStatValue(stats, "control") != 1 {
		t.Fatalf("expected 1 control frame in stats, got %#v", stats)
	}
	if websocketStatValue(stats, "fragmented") != 1 {
		t.Fatalf("expected 1 fragmented message in stats, got %#v", stats)
	}
}

func websocketStatValue(stats map[string]interface{}, key string) int {
	switch v := stats[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}
