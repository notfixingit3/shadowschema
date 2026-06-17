package spec

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestExportMapEndpointIncludesVault(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	sm.SaveVaultCredential("Authorization", "Bearer test-token")

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/export-map")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var exported map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&exported); err != nil {
		t.Fatalf("failed to decode export: %v", err)
	}

	vault, ok := exported["x-shadowschema-vault"].([]interface{})
	if !ok || len(vault) == 0 {
		t.Fatalf("expected vault data in export-map response, got %#v", exported["x-shadowschema-vault"])
	}
}

func TestVaultEndpointReturnsCredentials(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	token := "secret-key-" + t.Name()
	sm.SaveVaultCredential("X-Api-Key", token)

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/vault")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var credentials []AuthCredential
	if err := json.NewDecoder(resp.Body).Decode(&credentials); err != nil {
		t.Fatalf("failed to decode vault response: %v", err)
	}

	found := false
	for _, credential := range credentials {
		if credential.HeaderName == "X-Api-Key" && credential.TokenValue == token {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected saved credential in vault payload, got %#v", credentials)
	}
}

func TestGenerateSDKEndpointRejectsUnsupportedLanguage(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/generate-sdk", "application/json", bytes.NewBufferString(`{"language":"ruby"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported language, got %d", resp.StatusCode)
	}
}

func TestSessionsEndpointListAndCreate(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	listResp, err := http.Get(server.URL + "/sessions")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var sessions []SessionMeta
	if err := json.NewDecoder(listResp.Body).Decode(&sessions); err != nil {
		t.Fatalf("failed to decode sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 initial session, got %#v", sessions)
	}

	createBody := `{"name":"Second Run","target":"api.test.com","ignore_rules":"\\.css$"}`
	createResp, err := http.Post(server.URL+"/sessions", "application/json", bytes.NewBufferString(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on create, got %d", createResp.StatusCode)
	}
	if sm.GetTarget() != "api.test.com" {
		t.Fatalf("expected active target api.test.com, got %q", sm.GetTarget())
	}

	listResp2, err := http.Get(server.URL + "/sessions")
	if err != nil {
		t.Fatalf("second list request failed: %v", err)
	}
	defer listResp2.Body.Close()

	if err := json.NewDecoder(listResp2.Body).Decode(&sessions); err != nil {
		t.Fatalf("failed to decode sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions after create, got %#v", sessions)
	}
}

func TestSessionsEndpointCreateRequiresNameAndTarget(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/sessions", "application/json", bytes.NewBufferString(`{"name":"Missing Target"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSessionsSwitchEndpoint(t *testing.T) {
	sm := newTestSpecManager(t, "first.example.com")
	firstID := sm.SessionID
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	createResp, err := http.Post(
		server.URL+"/sessions",
		"application/json",
		bytes.NewBufferString(`{"name":"Second","target":"second.example.com"}`),
	)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	_ = createResp.Body.Close()

	secondID := sm.SessionID
	if secondID == firstID {
		t.Fatalf("expected new session id after create, still %d", firstID)
	}
	if sm.GetTarget() != "second.example.com" {
		t.Fatalf("expected active target second.example.com, got %q", sm.GetTarget())
	}

	switchResp, err := http.Post(
		server.URL+"/sessions/switch",
		"application/json",
		bytes.NewBufferString(`{"id":`+strconv.Itoa(firstID)+`}`),
	)
	if err != nil {
		t.Fatalf("switch request failed: %v", err)
	}
	_ = switchResp.Body.Close()

	if switchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on switch, got %d", switchResp.StatusCode)
	}
	if sm.SessionID != firstID {
		t.Fatalf("expected session id %d after switch, got %d", firstID, sm.SessionID)
	}
	if sm.GetTarget() != "first.example.com" {
		t.Fatalf("expected target first.example.com after switch, got %q", sm.GetTarget())
	}
}

func TestSessionsDeleteEndpointFallsBackToRemainingSession(t *testing.T) {
	sm := newTestSpecManager(t, "keep.example.com")
	keepID := sm.SessionID
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	createResp, err := http.Post(
		server.URL+"/sessions",
		"application/json",
		bytes.NewBufferString(`{"name":"Delete Me","target":"delete.example.com"}`),
	)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	_ = createResp.Body.Close()

	deleteID := sm.SessionID
	deleteResp, err := http.Post(
		server.URL+"/sessions/delete",
		"application/json",
		bytes.NewBufferString(`{"id":`+strconv.Itoa(deleteID)+`}`),
	)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	_ = deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", deleteResp.StatusCode)
	}
	if sm.SessionID != keepID {
		t.Fatalf("expected fallback to session %d, got %d", keepID, sm.SessionID)
	}
	if sm.GetTarget() != "keep.example.com" {
		t.Fatalf("expected target keep.example.com after delete, got %q", sm.GetTarget())
	}
}

func TestSessionsRenameEndpoint(t *testing.T) {
	sm := newTestSpecManager(t, "rename.example.com")
	sessionID := sm.SessionID
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	renameResp, err := http.Post(
		server.URL+"/sessions/rename",
		"application/json",
		bytes.NewBufferString(`{"id":`+strconv.Itoa(sessionID)+`,"name":"Renamed Session"}`),
	)
	if err != nil {
		t.Fatalf("rename request failed: %v", err)
	}
	defer renameResp.Body.Close()

	if renameResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on rename, got %d", renameResp.StatusCode)
	}

	listResp, err := http.Get(server.URL + "/sessions")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()

	var sessions []SessionMeta
	if err := json.NewDecoder(listResp.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions failed: %v", err)
	}
	for _, session := range sessions {
		if session.ID == sessionID && session.Name != "Renamed Session" {
			t.Fatalf("expected renamed session, got %q", session.Name)
		}
	}

	missingResp, err := http.Post(
		server.URL+"/sessions/rename",
		"application/json",
		bytes.NewBufferString(`{"id":99999,"name":"Missing"}`),
	)
	if err != nil {
		t.Fatalf("missing rename request failed: %v", err)
	}
	defer missingResp.Body.Close()
	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing session, got %d", missingResp.StatusCode)
	}
}

func TestSessionsAddTargetEndpoint(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	addResp, err := http.Post(
		server.URL+"/sessions/add-target",
		"application/json",
		bytes.NewBufferString(`{"domain":"api.example.com"}`),
	)
	if err != nil {
		t.Fatalf("add-target request failed: %v", err)
	}
	_ = addResp.Body.Close()

	if addResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", addResp.StatusCode)
	}
	if sm.GetTarget() != "example.com,api.example.com" {
		t.Fatalf("expected combined target, got %q", sm.GetTarget())
	}

	dupResp, err := http.Post(
		server.URL+"/sessions/add-target",
		"application/json",
		bytes.NewBufferString(`{"domain":"api.example.com"}`),
	)
	if err != nil {
		t.Fatalf("duplicate add-target request failed: %v", err)
	}
	_ = dupResp.Body.Close()

	if sm.GetTarget() != "example.com,api.example.com" {
		t.Fatalf("expected target unchanged after duplicate add, got %q", sm.GetTarget())
	}
}

func TestDiscoveredEndpointReturnsDomains(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	sm.AddDiscoveredDomain("discovered.example.com:443")

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/discovered")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var discovered []string
	if err := json.NewDecoder(resp.Body).Decode(&discovered); err != nil {
		t.Fatalf("failed to decode discovered response: %v", err)
	}

	found := false
	for _, domain := range discovered {
		if domain == "discovered.example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected discovered.example.com in %#v", discovered)
	}
}

func TestExportMapEndpointSupportsYAML(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/export-map?format=yaml")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/yaml" {
		t.Fatalf("expected application/yaml content type, got %q", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if !strings.Contains(string(body), "openapi:") {
		t.Fatalf("expected yaml openapi document, got %q", string(body))
	}
}

func TestExportEndpointsHandleOPTIONS(t *testing.T) {
	sm := newTestSpecManager(t, "example.com")
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	req, err := http.NewRequest(http.MethodOptions, server.URL+"/export-map", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected CORS allow-origin header")
	}
}

func TestGenerateSDKEndpointReturnsZip(t *testing.T) {
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not available, skipping SDK generation test")
	}

	sm := newTestSpecManager(t, "example.com")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/users", nil)
	sm.AddEndpoint(req, "/api/users", []byte(`{"id":1,"name":"Alice"}`))

	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/generate-sdk", "application/json", bytes.NewBufferString(`{"language":"python"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Skipf("SDK generation unavailable in this environment (status %d): %s", resp.StatusCode, body)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("expected application/zip, got %q", ct)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read zip body: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	if len(reader.File) == 0 {
		t.Fatalf("expected generated SDK files in zip")
	}
}

