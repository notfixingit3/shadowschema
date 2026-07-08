package spec

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"shadowschema/internal/router"
)

// HAR 1.2 subset used for offline recon import.
type harDocument struct {
	Log harLog `json:"log"`
}

type harLog struct {
	Entries []harEntry `json:"entries"`
}

type harEntry struct {
	Request  harRequest  `json:"request"`
	Response harResponse `json:"response"`
}

type harRequest struct {
	Method      string          `json:"method"`
	URL         string          `json:"url"`
	Headers     []harNameValue  `json:"headers"`
	QueryString []harNameValue  `json:"queryString"`
	PostData    *harPostData    `json:"postData"`
}

type harResponse struct {
	Status  int         `json:"status"`
	Headers []harNameValue `json:"headers"`
	Content harContent  `json:"content"`
}

type harNameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type harContent struct {
	Text     string `json:"text"`
	Encoding string `json:"encoding"`
	MimeType string `json:"mimeType"`
}

// HARImportResult summarizes an import-har run.
type HARImportResult struct {
	Imported       int      `json:"imported"`
	Skipped        int      `json:"skipped"`
	SkippedReasons []string `json:"skipped_reasons,omitempty"`
	SessionID      int      `json:"session_id"`
	Target         string   `json:"target"`
}

// ImportHAR parses a HAR 1.2 document and maps matching HTTP entries into the
// active session (same pipeline as live MITM: path dedupe, schemas, vault, GraphQL).
func (s *SpecManager) ImportHAR(r io.Reader, onlyMatchingTarget bool) (HARImportResult, error) {
	var doc harDocument
	dec := json.NewDecoder(r)
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return HARImportResult{}, fmt.Errorf("invalid HAR JSON: %w", err)
	}
	if len(doc.Log.Entries) == 0 {
		return HARImportResult{}, fmt.Errorf("HAR contains no entries")
	}

	result := HARImportResult{
		SessionID: s.SessionID,
		Target:    s.GetTarget(),
	}

	for i, entry := range doc.Log.Entries {
		if err := s.importHAREntry(entry, onlyMatchingTarget, &result); err != nil {
			result.Skipped++
			if len(result.SkippedReasons) < 20 {
				result.SkippedReasons = append(result.SkippedReasons, fmt.Sprintf("entry[%d]: %s", i, err.Error()))
			}
		}
	}

	return result, nil
}

func (s *SpecManager) importHAREntry(entry harEntry, onlyMatchingTarget bool, result *HARImportResult) error {
	rawURL := strings.TrimSpace(entry.Request.URL)
	if rawURL == "" {
		return fmt.Errorf("missing request url")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("bad url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}

	host := parsed.Hostname()
	if onlyMatchingTarget && !s.IsTarget(host) {
		return fmt.Errorf("host %q outside session target", host)
	}

	method := strings.ToUpper(strings.TrimSpace(entry.Request.Method))
	if method == "" {
		method = http.MethodGet
	}

	// Build an http.Request so AddEndpoint can read method, query, and headers.
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return err
	}
	for _, h := range entry.Request.Headers {
		name := strings.TrimSpace(h.Name)
		if name == "" || strings.EqualFold(name, "Content-Length") {
			continue
		}
		req.Header.Add(name, h.Value)
		// Mirror live proxy vault capture for common auth headers + Cookie.
		canonical := http.CanonicalHeaderKey(name)
		if isVaultAuthHeader(canonical) || canonical == "Cookie" {
			if strings.TrimSpace(h.Value) != "" {
				s.SaveVaultCredential(canonical, h.Value, host)
			}
		}
	}
	for _, q := range entry.Request.QueryString {
		if q.Name == "" {
			continue
		}
		values := req.URL.Query()
		values.Add(q.Name, q.Value)
		req.URL.RawQuery = values.Encode()
	}

	var requestBody []byte
	if entry.Request.PostData != nil && entry.Request.PostData.Text != "" {
		requestBody = []byte(entry.Request.PostData.Text)
	}

	responseBody := decodeHARContent(entry.Response.Content)
	for _, h := range entry.Response.Headers {
		if strings.EqualFold(h.Name, "Set-Cookie") && strings.TrimSpace(h.Value) != "" {
			s.SaveVaultCredential("Set-Cookie", h.Value, host)
		}
	}

	status := entry.Response.Status
	if status <= 0 {
		status = http.StatusOK
	}

	// Match live proxy filters: map 2xx always; 4xx/5xx only with body.
	switch {
	case status >= 200 && status < 300:
		// ok
	case status >= 400 && status < 600 && len(responseBody) > 0:
		// ok
	default:
		return fmt.Errorf("status %d skipped", status)
	}

	// Out-of-target hosts: optionally expand perimeter when not filtering.
	if !onlyMatchingTarget && !s.IsTarget(host) {
		s.AddDiscoveredDomain(host)
	}

	path := router.DeduplicatePath(parsed.Path)
	if path == "" {
		path = "/"
	}
	s.AddEndpoint(req, path, status, responseBody, requestBody)
	result.Imported++
	return nil
}

func decodeHARContent(c harContent) []byte {
	if c.Text == "" {
		return nil
	}
	if strings.EqualFold(c.Encoding, "base64") {
		if decoded, err := base64.StdEncoding.DecodeString(c.Text); err == nil {
			return decoded
		}
	}
	return []byte(c.Text)
}

func isVaultAuthHeader(canonical string) bool {
	switch http.CanonicalHeaderKey(canonical) {
	case "Authorization", "X-Api-Key", "X-Auth-Token", "Session-Token",
		"X-Access-Token", "X-Csrf-Token", "X-Xsrf-Token", "Api-Key",
		"X-Api-Token", "X-Session-Token", "X-Token":
		return true
	default:
		return false
	}
}

func (s *SpecManager) mountHARImportRoute(mux *http.ServeMux) {
	mux.HandleFunc("/import-har", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Cap body size (50MB matches nginx client_max_body_size).
		r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

		onlyMatching := true
		switch strings.ToLower(r.URL.Query().Get("only_matching_target")) {
		case "0", "false", "no":
			onlyMatching = false
		}

		// Accept raw HAR JSON body, or multipart file field "har" / "file".
		var reader io.Reader = r.Body
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "multipart/") {
			// Body already capped by MaxBytesReader above; maxMemory also bounds in-memory parts.
			// #nosec G120 -- maxMemory is 50 MiB and request body is MaxBytesReader-limited
			if err := r.ParseMultipartForm(50 << 20); err != nil {
				http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
				return
			}
			file, _, err := r.FormFile("har")
			if err != nil {
				file, _, err = r.FormFile("file")
			}
			if err != nil {
				http.Error(w, "multipart field 'har' or 'file' required", http.StatusBadRequest)
				return
			}
			defer file.Close()
			reader = file
			if v := r.FormValue("only_matching_target"); v != "" {
				switch strings.ToLower(v) {
				case "0", "false", "no":
					onlyMatching = false
				case "1", "true", "yes":
					onlyMatching = true
				}
			}
		}

		result, err := s.ImportHAR(reader, onlyMatching)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Persist immediately after bulk import.
		s.Flush()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
}
