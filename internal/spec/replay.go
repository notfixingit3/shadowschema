package spec

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type replayRequest struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

func (s *SpecManager) mountReplayRoute(mux *http.ServeMux) {
	mux.HandleFunc("/export-replay", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		var req replayRequest
		switch r.Method {
		case http.MethodGet:
			req.Path = r.URL.Query().Get("path")
			req.Method = r.URL.Query().Get("method")
		case http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if req.Path == "" || req.Method == "" {
			http.Error(w, "path and method required", http.StatusBadRequest)
			return
		}

		sessionID, explicit, err := parseSessionIDQuery(r.URL.Query().Get("session_id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		view, err := s.sessionReadView(sessionID, explicit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		detail, ok := endpointDetailFromDoc(view.Doc, req.Path)
		if !ok {
			http.Error(w, "endpoint not found", http.StatusNotFound)
			return
		}

		methodKey := strings.ToLower(req.Method)
		operations, ok := detail["operations"].(map[string]interface{})
		if !ok || operations[methodKey] == nil {
			http.Error(w, "method not found for endpoint", http.StatusNotFound)
			return
		}

		operation, ok := operations[methodKey].(map[string]interface{})
		if !ok {
			http.Error(w, "invalid operation payload", http.StatusInternalServerError)
			return
		}

		credentials, _ := s.listVaultCredentialsForSession(view.SessionID)
		script := buildPythonReplayScript(req.Path, strings.ToUpper(req.Method), operation, view.Target, credentials)

		w.Header().Set("Content-Type", "text/x-python; charset=utf-8")
		_, _ = w.Write([]byte(script))
	})
}

func buildPythonReplayScript(path, method string, operation map[string]interface{}, target string, credentials []AuthCredential) string {
	headers := map[string]string{
		"User-Agent": "ShadowSchema-Replay/1.0",
	}
	for _, credential := range credentials {
		if credential.HeaderName != "" && credential.TokenValue != "" {
			headers[credential.HeaderName] = credential.TokenValue
		}
	}

	url := buildReplayURL(path, target)
	headersJSON, _ := json.MarshalIndent(headers, "", "    ")

	var b strings.Builder
	if len(credentials) > 0 {
		b.WriteString("# Auth headers auto-injected from ShadowSchema Auth Vault\n")
	} else {
		b.WriteString("# No Auth Vault credentials captured yet for this session\n")
	}

	b.WriteString("import requests\nimport json\n\n")
	b.WriteString(fmt.Sprintf("url = %q\n\n", url))
	b.WriteString(fmt.Sprintf("headers = %s\n\n", string(headersJSON)))

	payloadKwarg := ""
	if payload, ok := operation["x-last-payload"]; ok && (method == "POST" || method == "PUT" || method == "PATCH") {
		payloadJSON, _ := json.MarshalIndent(payload, "", "    ")
		b.WriteString(fmt.Sprintf("payload = %s\n\n", string(payloadJSON)))
		payloadKwarg = ", json=payload"
	}

	b.WriteString(fmt.Sprintf("response = requests.request(%q, url, headers=headers%s)\n\n", method, payloadKwarg))
	b.WriteString("print(f\"Status: {response.status_code}\")\n")
	b.WriteString("print(response.text)\n")
	return b.String()
}

func buildReplayURL(path, target string) string {
	host := strings.TrimSpace(strings.Split(target, ",")[0])
	if host == "" {
		host = "target-domain.com"
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	return strings.TrimRight(host, "/") + path
}