package proxy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lqqyt2423/go-mitmproxy/cert"
)

// InitCA checks for existing CA files, migrates old ca.crt/ca.key if present,
// and ensures the cert directory and CA files are initialized using go-mitmproxy/cert.
func InitCA(certDir string) error {
	if err := os.MkdirAll(certDir, 0750); err != nil {
		return err
	}

	caCertPath := filepath.Join(certDir, "mitmproxy-ca-cert.pem")
	caKeyPath := filepath.Join(certDir, "mitmproxy-ca.pem")
	caCerPath := filepath.Join(certDir, "mitmproxy-ca-cert.cer")

	// Migrate old ca.crt and ca.key if they exist and new CA files do not
	oldCertPath := filepath.Join(certDir, "ca.crt")
	oldKeyPath := filepath.Join(certDir, "ca.key")

	_, errCert := os.Stat(caCertPath)
	_, errKey := os.Stat(caKeyPath)
	if os.IsNotExist(errCert) || os.IsNotExist(errKey) {
		if _, errOldCert := os.Stat(oldCertPath); errOldCert == nil {
			if _, errOldKey := os.Stat(oldKeyPath); errOldKey == nil {
				certBytes, err := os.ReadFile(oldCertPath)
				if err == nil {
					keyBytes, err := os.ReadFile(oldKeyPath)
					if err == nil {
						_ = os.WriteFile(caCertPath, certBytes, 0600)
						_ = os.WriteFile(caCerPath, certBytes, 0600)

						// Combine private key and certificate into mitmproxy-ca.pem
						combined := append(keyBytes, '\n')
						combined = append(combined, certBytes...)
						_ = os.WriteFile(caKeyPath, combined, 0600)

						fmt.Println("[INFO] Successfully migrated old CA to go-mitmproxy format")
					}
				}
			}
		}
	}

	// Let go-mitmproxy load or generate the CA certificates
	_, err := cert.NewSelfSignCA(certDir)
	return err
}
