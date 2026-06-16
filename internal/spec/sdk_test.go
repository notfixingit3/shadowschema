package spec

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeSDKLanguage(t *testing.T) {
	t.Parallel()

	language, err := normalizeSDKLanguage("")
	if err != nil || language != "python" {
		t.Fatalf("expected default python, got %q err=%v", language, err)
	}

	if _, err := normalizeSDKLanguage("typescript-fetch"); err != nil {
		t.Fatalf("expected supported language, got %v", err)
	}

	if _, err := normalizeSDKLanguage("ruby"); err == nil {
		t.Fatalf("expected unsupported language error")
	}
}

func TestZipDirectoryUsesRootScopedReads(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "client.go"), []byte("package client"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	archive, err := zipDirectory(root)
	if err != nil {
		t.Fatalf("zipDirectory failed: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("zip reader failed: %v", err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != "pkg/client.go" {
		t.Fatalf("unexpected zip contents: %#v", reader.File)
	}
}