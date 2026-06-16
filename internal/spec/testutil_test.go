package spec

import (
	"os"
	"testing"
)

func setupIsolatedDB(t *testing.T) {
	t.Helper()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
}

func newTestSpecManager(t *testing.T, target string) *SpecManager {
	t.Helper()
	setupIsolatedDB(t)
	return NewSpecManager(target)
}