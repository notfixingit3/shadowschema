package spec

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestSessionDiffAndValidateEndpoints(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	fromID := sm.SessionID

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/a", nil)
	sm.AddEndpoint(req, "/api/a", 200, []byte(`{"a":1}`), nil)
	sm.Flush() // persist before session switch so diff can load from DB

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	// Create second session with different paths.
	createResp, err := http.Post(
		server.URL+"/sessions",
		"application/json",
		bytes.NewBufferString(`{"name":"To Session","target":"example.com"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = createResp.Body.Close()
	toID := sm.SessionID
	if toID == fromID {
		t.Fatal("expected new session id")
	}

	req2, _ := http.NewRequest(http.MethodGet, "http://example.com/api/b", nil)
	sm.AddEndpoint(req2, "/api/b", 200, []byte(`{"b":1}`), nil)
	req3, _ := http.NewRequest(http.MethodPost, "http://example.com/api/a", nil)
	sm.AddEndpoint(req3, "/api/a", 201, []byte(`{"ok":true}`), []byte(`{}`))
	sm.Flush()

	diffResp, err := http.Get(server.URL + "/sessions/diff?from=" + strconv.Itoa(fromID) + "&to=" + strconv.Itoa(toID))
	if err != nil {
		t.Fatal(err)
	}
	defer diffResp.Body.Close()
	if diffResp.StatusCode != http.StatusOK {
		t.Fatalf("diff status %d", diffResp.StatusCode)
	}
	var diff SessionDiffResult
	if err := json.NewDecoder(diffResp.Body).Decode(&diff); err != nil {
		t.Fatal(err)
	}
	// from: /api/a GET; to: /api/a GET+POST, /api/b GET
	if len(diff.AddedPaths) == 0 && len(diff.ChangedPaths) == 0 {
		t.Fatalf("expected diff to report changes, got %+v", diff)
	}
	foundB := false
	for _, p := range diff.AddedPaths {
		if p == "/api/b" {
			foundB = true
		}
	}
	if !foundB {
		t.Fatalf("expected /api/b in added_paths, got %+v", diff.AddedPaths)
	}

	valResp, err := http.Get(server.URL + "/validate-spec")
	if err != nil {
		t.Fatal(err)
	}
	defer valResp.Body.Close()
	var validation SpecValidationResult
	if err := json.NewDecoder(valResp.Body).Decode(&validation); err != nil {
		t.Fatal(err)
	}
	if validation.PathCount < 1 {
		t.Fatalf("expected paths in validation, got %+v", validation)
	}
	if validation.OpCount < 1 {
		t.Fatalf("expected operations in validation, got %+v", validation)
	}
}
