package spec

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSpecManagerAddEndpoint(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

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
	sm := newTestSpecManager(t, "example.com")

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
	sm := newTestSpecManager(t, "example.com")
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

func TestAddEndpointIgnoresStaticAssets(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/static/logo.png", nil)
	sm.AddEndpoint(req, "/static/logo.png", []byte(`fake image bytes`))

	if sm.doc.Paths.Find("/static/logo.png") != nil {
		t.Fatalf("expected static asset path to be ignored")
	}
}

func TestAddEndpointSupportsPOSTMethod(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	req, _ := http.NewRequest(http.MethodPost, "http://example.com/api/items", nil)
	sm.AddEndpoint(req, "/api/items", []byte(`{"id": 42}`))

	pathItem := sm.doc.Paths.Find("/api/items")
	if pathItem == nil || pathItem.Post == nil {
		t.Fatalf("expected POST operation for /api/items")
	}
}

func TestAddEndpointStoresNonJSONPayload(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/raw", nil)
	sm.AddEndpoint(req, "/api/raw", []byte(`plain text response`))

	pathItem := sm.doc.Paths.Find("/api/raw")
	if pathItem == nil || pathItem.Get == nil {
		t.Fatalf("expected GET operation for /api/raw")
	}

	payload, ok := pathItem.Get.Extensions["x-last-payload"].(string)
	if !ok || payload != "plain text response" {
		t.Fatalf("expected string x-last-payload, got %#v", pathItem.Get.Extensions["x-last-payload"])
	}
}

func TestAddEndpointCapturesQueryAndHeaderParams(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/search?q=shadow&limit=10", nil)
	req.Header.Set("X-Request-Id", "abc-123")
	sm.AddEndpoint(req, "/api/search", []byte(`{"results":[]}`))

	pathItem := sm.doc.Paths.Find("/api/search")
	if pathItem == nil || pathItem.Get == nil {
		t.Fatalf("expected GET operation for /api/search")
	}

	params := make(map[string]string)
	for _, p := range pathItem.Get.Parameters {
		if p.Value != nil {
			params[p.Value.Name] = p.Value.In
		}
	}

	if params["q"] != "query" {
		t.Fatalf("expected query param q, got %#v", params)
	}
	if params["limit"] != "query" {
		t.Fatalf("expected query param limit, got %#v", params)
	}
	if params["X-Request-Id"] != "header" {
		t.Fatalf("expected X-Request-Id header param, got %#v", params)
	}
}

func TestIsTargetMatchesCommaSeparatedDomains(t *testing.T) {
	sm := newTestSpecManager(t, "example.com,api.example.com")

	if !sm.IsTarget("api.example.com:443") {
		t.Fatalf("expected api.example.com to match target list")
	}
	if sm.IsTarget("unrelated.io") {
		t.Fatalf("expected unrelated.io not to match target list")
	}
}

func TestAddDiscoveredDomainDeduplicatesHosts(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	sm.AddDiscoveredDomain("new.example.com:443")
	sm.AddDiscoveredDomain("new.example.com:8443")

	if len(sm.Discovered) != 1 {
		t.Fatalf("expected 1 discovered host, got %d", len(sm.Discovered))
	}
	if !sm.Discovered["new.example.com"] {
		t.Fatalf("expected new.example.com to be discovered")
	}
}

func TestDiscoveredDomainsPersistAcrossReload(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	sessionID := sm.SessionID

	sm.AddDiscoveredDomain("shadow.example.com:443")
	sm.AddDiscoveredDomain("other.example.com:8443")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err := sm.dbQueryRow(
			`SELECT COUNT(*) FROM discovered_domains WHERE session_id = ?`,
			sessionID,
		).Scan(&count); err == nil && count == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	reloaded := NewSpecManager("example.com")
	if reloaded.SessionID != sessionID {
		t.Fatalf("expected session %d, got %d", sessionID, reloaded.SessionID)
	}
	if len(reloaded.Discovered) != 2 {
		t.Fatalf("expected 2 discovered hosts after reload, got %d", len(reloaded.Discovered))
	}
	if !reloaded.Discovered["shadow.example.com"] || !reloaded.Discovered["other.example.com"] {
		t.Fatalf("expected persisted domains in %#v", reloaded.Discovered)
	}
}

func TestExportJSONWritesSpecFile(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/ping", nil)
	sm.AddEndpoint(req, "/api/ping", []byte(`{"ok":true}`))

	filename := filepath.Join(t.TempDir(), "openapi-test.json")
	if err := sm.ExportJSON(filename); err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	if !strings.Contains(string(data), "/api/ping") {
		t.Fatalf("expected exported spec to include /api/ping, got %q", string(data))
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
