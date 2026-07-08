package spec

import (
	"net/http"
	"os"
	"strings"
)

// defaultCORSOrigins used when SHADOWSCHEMA_CORS_ORIGINS is unset.
// Same-origin nginx proxy needs no CORS; these cover local dashboard/dev.
var defaultCORSOrigins = []string{
	"http://localhost:8080",
	"http://127.0.0.1:8080",
	"http://localhost:8082",
	"http://127.0.0.1:8082",
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

func corsAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("SHADOWSCHEMA_CORS_ORIGINS"))
	if raw == "" {
		return defaultCORSOrigins
	}
	if raw == "*" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func exportAPIToken() string {
	return strings.TrimSpace(os.Getenv("SHADOWSCHEMA_EXPORT_TOKEN"))
}

func enableCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	allowed := corsAllowedOrigins()
	if len(allowed) == 1 && allowed[0] == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-ShadowSchema-Token")
		return
	}

	for _, a := range allowed {
		if a == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-ShadowSchema-Token")
			return
		}
	}
}

func requestHasExportToken(r *http.Request) bool {
	token := exportAPIToken()
	if token == "" {
		return true
	}
	if got := strings.TrimSpace(r.Header.Get("X-ShadowSchema-Token")); got != "" && got == token {
		return true
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		got := strings.TrimSpace(auth[7:])
		return got == token
	}
	// Do not accept tokens via query string — they leak into access logs and Referer.
	return false
}

// withExportAuth wraps handlers to require SHADOWSCHEMA_EXPORT_TOKEN when set.
// OPTIONS preflight and unauthenticated health probes are allowed without a token
// only when no token is configured; when configured, all non-OPTIONS need it.
func withExportAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if !requestHasExportToken(r) {
			http.Error(w, "unauthorized: provide Authorization Bearer or X-ShadowSchema-Token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
