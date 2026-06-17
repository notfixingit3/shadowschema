package spec

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestScheduleSaveDebouncesDatabaseWrites(t *testing.T) {
	t.Setenv("SHADOWSCHEMA_SAVE_DEBOUNCE_MS", "200")

	sm := newTestSpecManager(t, "example.com")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/api/users", nil)

	var before string
	if err := sm.db.QueryRow(`SELECT spec_json FROM sessions WHERE id = ?`, sm.SessionID).Scan(&before); err != nil {
		t.Fatalf("read initial spec failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		sm.AddEndpoint(req, "/api/users", []byte(`{"id":1}`))
	}

	time.Sleep(50 * time.Millisecond)

	var pending string
	if err := sm.db.QueryRow(`SELECT spec_json FROM sessions WHERE id = ?`, sm.SessionID).Scan(&pending); err != nil {
		t.Fatalf("read pending spec failed: %v", err)
	}
	if pending != before {
		t.Fatalf("expected debounced save to delay DB write")
	}

	sm.Flush()

	var after string
	if err := sm.db.QueryRow(`SELECT spec_json FROM sessions WHERE id = ?`, sm.SessionID).Scan(&after); err != nil {
		t.Fatalf("read flushed spec failed: %v", err)
	}
	if !strings.Contains(after, "/api/users") {
		t.Fatalf("expected flushed spec to include endpoint, got %q", after)
	}
}