package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCAGeneratesAndReloads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := InitCA(dir); err != nil {
		t.Fatalf("InitCA failed on first run: %v", err)
	}

	for _, name := range []string{"ca.crt", "ca.key"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}

	if err := InitCA(dir); err != nil {
		t.Fatalf("InitCA failed when loading existing CA: %v", err)
	}
}