package spec

import (
	"net/http"
	"os"
	"path/filepath"
)

func certDir() string {
	if dir := os.Getenv("SHADOWSCHEMA_CERT_DIR"); dir != "" {
		return dir
	}
	return "certs"
}

func (s *SpecManager) mountCACertRoute(mux *http.ServeMux) {
	mux.HandleFunc("/ca-cert", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		certPath := filepath.Join(certDir(), "ca.crt")
		// #nosec G304 -- path is constrained to certDir()/ca.crt
		data, err := os.ReadFile(certPath)
		if err != nil {
			http.Error(w, "CA certificate not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Content-Disposition", `attachment; filename="shadowschema-ca.crt"`)
		_, _ = w.Write(data)
	})
}