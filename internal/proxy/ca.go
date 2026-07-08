package proxy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/cert"
)

// InitCA checks for existing CA files, migrates old ca.crt/ca.key if present,
// and ensures the cert directory and CA files are initialized using go-mitmproxy/cert.
//
// certDir is cleaned and all cert reads/writes are scoped under that directory
// via os.Root to prevent path traversal.
func InitCA(certDir string) error {
	certDir = filepath.Clean(strings.TrimSpace(certDir))
	if certDir == "" || certDir == "." {
		certDir = "certs"
	}

	if err := os.MkdirAll(certDir, 0750); err != nil {
		return err
	}

	root, err := os.OpenRoot(certDir)
	if err != nil {
		return fmt.Errorf("open cert root: %w", err)
	}
	defer root.Close()

	const (
		caCertName = "mitmproxy-ca-cert.pem"
		caKeyName  = "mitmproxy-ca.pem"
		caCerName  = "mitmproxy-ca-cert.cer"
		oldCert    = "ca.crt"
		oldKey     = "ca.key"
	)

	// Migrate old ca.crt and ca.key if they exist and new CA files do not.
	if !rootExists(root, caCertName) || !rootExists(root, caKeyName) {
		if rootExists(root, oldCert) && rootExists(root, oldKey) {
			certBytes, err := rootReadFile(root, oldCert)
			if err == nil {
				keyBytes, err := rootReadFile(root, oldKey)
				if err == nil {
					_ = rootWriteFile(root, caCertName, certBytes, 0600)
					_ = rootWriteFile(root, caCerName, certBytes, 0600)

					// Combine private key and certificate into mitmproxy-ca.pem
					combined := append(keyBytes, '\n')
					combined = append(combined, certBytes...)
					_ = rootWriteFile(root, caKeyName, combined, 0600)

					fmt.Println("[INFO] Successfully migrated old CA to go-mitmproxy format")
				}
			}
		}
	}

	// Let go-mitmproxy load or generate the CA certificates (path is cleaned/owned).
	_, err = cert.NewSelfSignCA(certDir)
	return err
}

func rootExists(root *os.Root, name string) bool {
	_, err := root.Stat(name)
	return err == nil
}

func rootReadFile(root *os.Root, name string) ([]byte, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func rootWriteFile(root *os.Root, name string, data []byte, perm os.FileMode) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
