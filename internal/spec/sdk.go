package spec

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var allowedSDKLanguages = map[string]struct{}{
	"python":           {},
	"typescript-fetch": {},
}

func normalizeSDKLanguage(language string) (string, error) {
	if language == "" {
		return "python", nil
	}
	if _, ok := allowedSDKLanguages[language]; !ok {
		return "", fmt.Errorf("unsupported sdk language: %s", language)
	}
	return language, nil
}

func generateSDKZip(specData []byte, language string) ([]byte, error) {
	language, err := normalizeSDKLanguage(language)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "shadowschema-sdk-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, specData, 0o600); err != nil {
		return nil, err
	}

	outDir := filepath.Join(tmpDir, "out")
	// #nosec G204 -- generator language is allowlist-validated and paths are temp-dir scoped
	cmd := exec.Command("npx", "-y", "@openapitools/openapi-generator-cli", "generate", "-i", specFile, "-g", language, "-o", outDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("sdk generation failed: %w: %s", err, string(out))
	}

	return zipDirectory(outDir)
}

func zipDirectory(root string) ([]byte, error) {
	root = filepath.Clean(root)

	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer rootFS.Close()

	zipBuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuf)

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil || pathEscapesRoot(relPath) {
			return fmt.Errorf("invalid sdk output path: %s", path)
		}

		entry, err := zipWriter.Create(filepath.ToSlash(relPath))
		if err != nil {
			return err
		}

		f, err := rootFS.Open(relPath)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(entry, f)
		return err
	})
	if walkErr != nil {
		return nil, walkErr
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return zipBuf.Bytes(), nil
}

func pathEscapesRoot(relPath string) bool {
	return relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator))
}