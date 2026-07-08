package spec

import (
	"path/filepath"
	"testing"
)

func setupIsolatedDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "shadowschema.db")
	t.Setenv("SHADOWSCHEMA_DB_PATH", dbPath)
}

func newTestSpecManager(t *testing.T, target string) *SpecManager {
	t.Helper()
	setupIsolatedDB(t)
	sm := NewSpecManager(target)
	t.Cleanup(func() {
		// Wait for background writes, flush, and close so TempDir cleanup succeeds.
		_ = sm.Close()
	})
	return sm
}