package spec

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestHealthEndpointReturnsSessionMetadata(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/ping", nil)
	sm.AddEndpoint(req, "/api/ping", []byte(`{"ok":true}`))

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("expected ok status, got %q", health.Status)
	}
	if health.EndpointCount != 1 {
		t.Fatalf("expected 1 endpoint, got %d", health.EndpointCount)
	}
	if !health.ActiveSession {
		t.Fatalf("expected active session")
	}
}

func TestEndpointsIndexSupportsPathPrefix(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	getReq, _ := http.NewRequest(http.MethodGet, "http://example.com/api/users", nil)
	sm.AddEndpoint(getReq, "/api/users", []byte(`{"id":1}`))
	healthReq, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	sm.AddEndpoint(healthReq, "/health", []byte(`{"ok":true}`))

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/endpoints?path_prefix=/api")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload EndpointIndexResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload.Count != 1 {
		t.Fatalf("expected 1 endpoint, got %d", payload.Count)
	}
	if payload.Endpoints[0].Path != "/api/users" {
		t.Fatalf("unexpected endpoint: %#v", payload.Endpoints[0])
	}
	if !payload.Endpoints[0].HasPayload {
		t.Fatalf("expected has_payload true")
	}
	if payload.Endpoints[0].LastSeen == "" {
		t.Fatalf("expected last_seen to be populated")
	}
}

func TestEndpointDetailRouteReturnsOperations(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/users", nil)
	sm.AddEndpoint(req, "/api/users", []byte(`{"id":1}`))

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/endpoints/api/users")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var detail map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if detail["path"] != "/api/users" {
		t.Fatalf("unexpected path: %#v", detail["path"])
	}
	operations, ok := detail["operations"].(map[string]interface{})
	if !ok || operations["get"] == nil {
		t.Fatalf("expected get operation in %#v", detail)
	}
}

func TestExportMapSupportsPathPrefixAndSessionID(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	req1, _ := http.NewRequest(http.MethodGet, "http://example.com/api/a", nil)
	sm.AddEndpoint(req1, "/api/a", []byte(`{"a":1}`))
	req2, _ := http.NewRequest(http.MethodGet, "http://example.com/other", nil)
	sm.AddEndpoint(req2, "/other", []byte(`{"b":2}`))

	secondID, err := sm.insertSession("Second", "other.example.com", "", `{"openapi":"3.0.0","info":{"title":"t","version":"1"},"paths":{}}`)
	if err != nil {
		t.Fatalf("insert session failed: %v", err)
	}

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/export-map?path_prefix=/api")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var exported map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&exported); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	paths, ok := exported["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected paths map")
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 filtered path, got %#v", paths)
	}

	resp2, err := http.Get(server.URL + "/health?session_id=" + strconv.Itoa(secondID))
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp2.Body.Close()

	var health HealthResponse
	if err := json.NewDecoder(resp2.Body).Decode(&health); err != nil {
		t.Fatalf("decode health failed: %v", err)
	}
	if health.SessionID != secondID {
		t.Fatalf("expected session %d, got %d", secondID, health.SessionID)
	}
	if health.ActiveSession {
		t.Fatalf("expected inactive session read")
	}
	if health.EndpointCount != 0 {
		t.Fatalf("expected empty second session, got %d", health.EndpointCount)
	}
}

