package spec

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/getkin/kin-openapi/openapi3"
	"shadowschema/internal/parser"
	"shadowschema/internal/router"
)

type SpecManager struct {
	mu           sync.Mutex
	doc          *openapi3.T
	db           *sql.DB
	dbDriver     string
	SessionID    int
	TargetDomain string
	IgnoreRules  string
	Discovered   map[string]bool

	saveTimerMu sync.Mutex
	saveTimer   *time.Timer
}

type SessionMeta struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Target      string    `json:"target"`
	IgnoreRules string    `json:"ignore_rules"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AuthCredential struct {
	HeaderName string    `json:"header_name"`
	TokenValue string    `json:"token_value"`
	Host       string    `json:"host,omitempty"`
	FirstSeen  time.Time `json:"first_seen"`
}

func NewSpecManager(defaultTarget string) *SpecManager {
	db, driver, err := openDatabase()
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	sm := &SpecManager{db: db, dbDriver: driver, Discovered: make(map[string]bool)}
	sm.LoadLatestOrCreate(defaultTarget)
	return sm
}

func (s *SpecManager) LoadLatestOrCreate(target string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var specJSON string
	var id int
	var t string
	var ignore string

	err := s.dbQueryRow(`SELECT id, target, ignore_rules, spec_json FROM sessions ORDER BY updated_at DESC LIMIT 1`).Scan(&id, &t, &ignore, &specJSON)
	if err == nil && specJSON != "" {
		if doc, ok := s.loadAndMigrateSpec(id, specJSON); ok {
			s.doc = doc
			s.SessionID = id
			s.TargetDomain = t
			s.IgnoreRules = ignore
			s.Discovered = s.loadDiscoveredDomainsLocked(id)
			return
		}
	}

	// Create new
	s.doc = newEmptySpec(target)
	s.TargetDomain = target
	s.IgnoreRules = "\\.(png|jpg|jpeg|webp|gif|css|js|woff|woff2|ico)$"
	data, _ := json.Marshal(s.doc)
	if newID, err := s.insertSession("Initial Run", target, s.IgnoreRules, string(data)); err == nil {
		s.SessionID = newID
	}
}

func newEmptySpec(target string) *openapi3.T {
	doc := &openapi3.T{
		OpenAPI: "3.0.0",
		Info:    &openapi3.Info{Title: "ShadowSchema Auto-Generated API", Version: "1.0.0"},
		Paths:   openapi3.NewPaths(),
	}
	ensureServers(doc, target)
	return doc
}

func ensureServers(doc *openapi3.T, target string) {
	if doc == nil {
		return
	}
	url := serverURLForTarget(target)
	if len(doc.Servers) == 1 && doc.Servers[0] != nil && doc.Servers[0].URL == url {
		return
	}
	doc.Servers = openapi3.Servers{
		&openapi3.Server{URL: url, Description: "Primary target inferred by ShadowSchema"},
	}
}

func (s *SpecManager) GetTarget() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.TargetDomain
}

// SaveVaultCredential stores a captured auth header for the active session,
// scoped to the request host (empty host = session-global / legacy).
// Header names are canonicalized so live and HAR capture use the same keys.
func (s *SpecManager) SaveVaultCredential(headerName, tokenValue, host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.saveVaultCredential(http.CanonicalHeaderKey(headerName), tokenValue, normalizeHost(host)); err != nil {
		log.Printf("[WARN] vault save failed (header=%s host=%s): %v", headerName, host, err)
	}
}

func (s *SpecManager) IsTarget(host string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	targets := strings.Split(s.TargetDomain, ",")
	for _, t := range targets {
		if hostMatchesTarget(host, t) {
			return true
		}
	}
	return false
}

func (s *SpecManager) AddDiscoveredDomain(host string) {
	host = strings.Split(host, ":")[0]
	if host == "" {
		return
	}

	s.mu.Lock()
	if s.Discovered[host] {
		s.mu.Unlock()
		return
	}
	s.Discovered[host] = true
	sessionID := s.SessionID
	s.mu.Unlock()

	if sessionID <= 0 {
		return
	}

	go func() {
		_ = s.persistDiscoveredDomain(sessionID, host)
	}()
}

func (s *SpecManager) loadDiscoveredDomainsLocked(sessionID int) map[string]bool {
	discovered := make(map[string]bool)
	rows, err := s.dbQuery(`SELECT host FROM discovered_domains WHERE session_id = ? ORDER BY host`, sessionID)
	if err != nil {
		return discovered
	}
	defer rows.Close()

	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err == nil && host != "" {
			discovered[host] = true
		}
	}
	return discovered
}

func (s *SpecManager) persistDiscoveredDomain(sessionID int, host string) error {
	if s.dbDriver == driverPostgres {
		_, err := s.dbExec(
			`INSERT INTO discovered_domains (session_id, host) VALUES (?, ?) ON CONFLICT DO NOTHING`,
			sessionID, host,
		)
		return err
	}

	_, err := s.dbExec(
		`INSERT OR IGNORE INTO discovered_domains (session_id, host) VALUES (?, ?)`,
		sessionID, host,
	)
	return err
}

func (s *SpecManager) AddWebSocket(req *http.Request, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.IgnoreRules != "" {
		if matched, _ := regexp.MatchString(s.IgnoreRules, path); matched {
			return
		}
	}

	pathItem := s.doc.Paths.Find(path)
	if pathItem == nil {
		pathItem = &openapi3.PathItem{}
		s.doc.Paths.Set(path, pathItem)
	}

	if pathItem.Get == nil {
		pathItem.Get = openapi3.NewOperation()
	}
	operation := pathItem.Get

	if operation.Extensions == nil {
		operation.Extensions = make(map[string]interface{})
	}
	operation.Extensions["x-websocket"] = true
	if operation.Summary == "" {
		operation.Summary = "WebSocket Connection"
	}
	if operation.Description == "" {
		operation.Description = "Detected WebSocket upgrade on this endpoint."
	}

	ensurePathParameters(operation, path)

	for key := range req.URL.Query() {
		exists := false
		for _, p := range operation.Parameters {
			if p.Value != nil && p.Value.Name == key && p.Value.In == "query" {
				exists = true
				break
			}
		}
		if !exists {
			param := openapi3.NewQueryParameter(key)
			param.Schema = openapi3.NewSchemaRef("", openapi3.NewStringSchema())
			operation.AddParameter(param)
		}
	}

	for key := range req.Header {
		canonical := http.CanonicalHeaderKey(key)
		if !strings.HasPrefix(strings.ToLower(canonical), "sec-websocket-") {
			continue
		}
		exists := false
		for _, p := range operation.Parameters {
			if p.Value != nil && p.Value.Name == canonical && p.Value.In == "header" {
				exists = true
				break
			}
		}
		if !exists {
			param := openapi3.NewHeaderParameter(canonical)
			param.Schema = openapi3.NewSchemaRef("", openapi3.NewStringSchema())
			operation.AddParameter(param)
		}
	}

	s.scheduleSave()
}

// AddEndpoint records an observed HTTP exchange.
// statusCode is the real response status (stored under that key in OpenAPI).
// responseBody may be empty (e.g. 204 No Content).
// requestBody is optional and drives requestBody schema + replay scripts.
func (s *SpecManager) AddEndpoint(req *http.Request, path string, statusCode int, responseBody, requestBody []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.IgnoreRules != "" {
		if matched, _ := regexp.MatchString(s.IgnoreRules, path); matched {
			return
		}
	}

	if statusCode <= 0 {
		statusCode = http.StatusOK
	}

	pathItem := s.doc.Paths.Find(path)
	if pathItem == nil {
		pathItem = &openapi3.PathItem{}
		s.doc.Paths.Set(path, pathItem)
	}

	var operation *openapi3.Operation
	switch req.Method {
	case http.MethodGet:
		if pathItem.Get == nil {
			pathItem.Get = openapi3.NewOperation()
		}
		operation = pathItem.Get
	case http.MethodPost:
		if pathItem.Post == nil {
			pathItem.Post = openapi3.NewOperation()
		}
		operation = pathItem.Post
	case http.MethodPut:
		if pathItem.Put == nil {
			pathItem.Put = openapi3.NewOperation()
		}
		operation = pathItem.Put
	case http.MethodDelete:
		if pathItem.Delete == nil {
			pathItem.Delete = openapi3.NewOperation()
		}
		operation = pathItem.Delete
	case http.MethodPatch:
		if pathItem.Patch == nil {
			pathItem.Patch = openapi3.NewOperation()
		}
		operation = pathItem.Patch
	default:
		return
	}

	if operation.Responses == nil {
		operation.Responses = openapi3.NewResponses()
	}

	if operation.Extensions == nil {
		operation.Extensions = make(map[string]interface{})
	}

	operation.Extensions["x-last-seen"] = time.Now().UTC().Format(time.RFC3339)
	operation.Extensions["x-last-status"] = statusCode

	if len(responseBody) > 0 {
		operation.Extensions["x-last-payload"] = decodeJSONPayload(responseBody)
	}

	if len(requestBody) > 0 {
		operation.Extensions["x-last-request-body"] = decodeJSONPayload(requestBody)
		mergeRequestBodySchema(operation, requestBody)
	}

	mergeGraphQLMetadata(operation, path, requestBody, responseBody)

	ensurePathParameters(operation, path)

	ignoreHeaders := map[string]bool{
		"Host": true, "Connection": true, "Accept-Encoding": true, "User-Agent": true,
		"Accept": true, "Accept-Language": true, "Sec-Fetch-Mode": true, "Sec-Fetch-Site": true,
		"Sec-Fetch-Dest": true, "Referer": true, "Origin": true, "Content-Length": true,
		"Content-Type": true, "X-Forwarded-For": true, "X-Forwarded-Proto": true,
		"Sec-Ch-Ua": true, "Sec-Ch-Ua-Mobile": true, "Sec-Ch-Ua-Platform": true,
		// Cookie is captured in the Auth Vault, not as a free-form header parameter.
		"Cookie": true,
	}
	// Multi-sample history, hit counts, typed query params, required inference.
	recordObservationStats(operation, req, statusCode, responseBody, requestBody, ignoreHeaders)

	statusKey := strconv.Itoa(statusCode)
	newSchema := parser.ParseResponseBody(responseBody)
	resp := operation.Responses.Value(statusKey)
	if resp == nil {
		respValue := openapi3.NewResponse().WithDescription(fmt.Sprintf("Observed HTTP %d", statusCode))
		if newSchema != nil && newSchema.Value != nil {
			content := openapi3.NewContentWithJSONSchema(newSchema.Value)
			respValue = respValue.WithContent(content)
		}
		operation.Responses.Set(statusKey, &openapi3.ResponseRef{Value: respValue})
	} else if newSchema != nil && newSchema.Value != nil {
		if resp.Value.Content == nil {
			resp.Value.Content = openapi3.NewContent()
		}
		content := resp.Value.Content.Get("application/json")
		if content != nil && content.Schema != nil {
			content.Schema = parser.MergeSchema(content.Schema, newSchema)
		} else {
			mediaType := openapi3.NewMediaType()
			mediaType.Schema = newSchema
			resp.Value.Content["application/json"] = mediaType
		}
	}

	ensureServers(s.doc, s.TargetDomain)
	s.scheduleSave()
}

func decodeJSONPayload(body []byte) interface{} {
	var raw map[string]interface{}
	var rawArr []interface{}
	if err := json.Unmarshal(body, &raw); err == nil {
		return raw
	}
	if err := json.Unmarshal(body, &rawArr); err == nil {
		return rawArr
	}
	return string(body)
}

func mergeRequestBodySchema(operation *openapi3.Operation, requestBody []byte) {
	newSchema := parser.ParseRequestBody(requestBody)
	if newSchema == nil || newSchema.Value == nil {
		return
	}
	if operation.RequestBody == nil {
		mediaType := openapi3.NewMediaType()
		mediaType.Schema = newSchema
		rb := openapi3.NewRequestBody().WithDescription("Inferred from observed request body").WithContent(
			openapi3.NewContentWithJSONSchema(newSchema.Value),
		)
		operation.RequestBody = &openapi3.RequestBodyRef{Value: rb}
		return
	}
	if operation.RequestBody.Value == nil {
		return
	}
	if operation.RequestBody.Value.Content == nil {
		operation.RequestBody.Value.Content = openapi3.NewContent()
	}
	content := operation.RequestBody.Value.Content.Get("application/json")
	if content != nil && content.Schema != nil {
		content.Schema = parser.MergeSchema(content.Schema, newSchema)
	} else {
		mediaType := openapi3.NewMediaType()
		mediaType.Schema = newSchema
		operation.RequestBody.Value.Content["application/json"] = mediaType
	}
}

func ensurePathParameters(operation *openapi3.Operation, path string) {
	for _, pp := range router.PathParamsFromTemplate(path) {
		exists := false
		for _, p := range operation.Parameters {
			if p.Value != nil && p.Value.Name == pp.Name && p.Value.In == "path" {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		param := openapi3.NewPathParameter(pp.Name)
		param.Required = true
		schema := openapi3.NewStringSchema()
		if pp.Schema == "integer" {
			schema = openapi3.NewIntegerSchema()
		}
		if pp.Format != "" {
			schema.Format = pp.Format
		}
		param.Schema = openapi3.NewSchemaRef("", schema)
		operation.AddParameter(param)
	}
}

func (s *SpecManager) ExportJSON(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0600)
}

func (s *SpecManager) ExportHandler() http.Handler {
	mux := http.NewServeMux()
	s.mountExportRoutes(mux)
	return withExportAuth(mux)
}

func (s *SpecManager) StartExportServer(addr string) {
	if addr == "" {
		addr = exportListenAddr()
	}
	fmt.Printf("[INFO] Export server running on %s\n", addr)
	if token := exportAPIToken(); token != "" {
		fmt.Println("[INFO] Export API token auth enabled (SHADOWSCHEMA_EXPORT_TOKEN)")
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.ExportHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	_ = srv.ListenAndServe()
}

// exportListenAddr returns the bind address for the export API.
// SHADOWSCHEMA_EXPORT_ADDR overrides fully (e.g. "127.0.0.1:38081" or ":38081").
// Default is ":38081" so Docker/internal networking works; compose should publish
// host ports as 127.0.0.1:38081 only.
func exportListenAddr() string {
	if addr := strings.TrimSpace(os.Getenv("SHADOWSCHEMA_EXPORT_ADDR")); addr != "" {
		return addr
	}
	return ":38081"
}

func parseIncludeSecrets(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *SpecManager) mountExportRoutes(mux *http.ServeMux) {
	s.mountCACertRoute(mux)
	s.mountHealthAndEndpointRoutes(mux)
	s.mountReplayRoute(mux)
	s.mountHARImportRoute(mux)
	s.mountDiffAndValidateRoutes(mux)

	mux.HandleFunc("/export-map", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
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

		includeSecrets := parseIncludeSecrets(r.URL.Query().Get("include_secrets"))
		doc := filterDocByPathPrefix(view.Doc, r.URL.Query().Get("path_prefix"))
		s.mu.Lock()
		data, err := s.buildExportDocumentFrom(doc, view.SessionID, includeSecrets)
		s.mu.Unlock()

		if err != nil {
			http.Error(w, "Failed to marshal spec", http.StatusInternalServerError)
			return
		}

		format := r.URL.Query().Get("format")
		if format == "yaml" {
			var obj interface{}
			if err := json.Unmarshal(data, &obj); err == nil {
				if yamlData, err := yaml.Marshal(obj); err == nil {
					w.Header().Set("Content-Type", "application/yaml")
					_, _ = w.Write(yamlData)
					return
				}
			}
			http.Error(w, "Failed to convert to YAML", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "GET" {
			rows, err := s.dbQuery(`SELECT id, name, target, ignore_rules, updated_at FROM sessions ORDER BY updated_at DESC`)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var sessions []SessionMeta
			for rows.Next() {
				var sm SessionMeta
				if err := rows.Scan(&sm.ID, &sm.Name, &sm.Target, &sm.IgnoreRules, &sm.UpdatedAt); err == nil {
					sessions = append(sessions, sm)
				}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sessions)
			return
		}

		if r.Method == "POST" {
			var reqData struct {
				Name   string `json:"name"`
				Target string `json:"target"`
				Ignore string `json:"ignore_rules"`
			}
			if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			if reqData.Name == "" || reqData.Target == "" {
				http.Error(w, "Name and Target required", http.StatusBadRequest)
				return
			}

			// Persist in-flight endpoints for the previous active session first.
			s.Flush()

			s.mu.Lock()
			s.doc = newEmptySpec(reqData.Target)
			s.TargetDomain = reqData.Target
			s.IgnoreRules = reqData.Ignore
			data, _ := json.Marshal(s.doc)
			newID, err := s.insertSession(reqData.Name, reqData.Target, reqData.Ignore, string(data))
			if err != nil {
				s.mu.Unlock()
				http.Error(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if newID <= 0 {
				s.mu.Unlock()
				http.Error(w, "Failed to create session: invalid id", http.StatusInternalServerError)
				return
			}
			s.SessionID = newID
			s.Discovered = make(map[string]bool)
			s.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":   true,
				"id":   newID,
				"name": reqData.Name,
				"target": reqData.Target,
			})
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/discovered", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		s.mu.Lock()
		keys := make([]string, 0, len(s.Discovered))
		for k := range s.Discovered {
			keys = append(keys, k)
		}
		s.mu.Unlock()

		sort.Strings(keys)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(keys)
	})

	mux.HandleFunc("/vault", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" {
			// Default: redact token values. Pass include_values=1 for full secrets
			// (dashboard vault UI, local replay). Safer default if export API is exposed.
			includeValues := parseIncludeSecrets(r.URL.Query().Get("include_values"))
			hostFilter := strings.TrimSpace(r.URL.Query().Get("host"))

			credentials, err := s.listVaultCredentialsForSession(s.SessionID, hostFilter)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if !includeValues {
				credentials = redactCredentials(credentials)
			}

			w.Header().Set("Content-Type", "application/json")
			if !includeValues {
				w.Header().Set("X-ShadowSchema-Vault-Redacted", "1")
			}
			_ = json.NewEncoder(w).Encode(credentials)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/sessions/add-target", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "POST" {
			var reqData struct {
				Domain string `json:"domain"`
			}
			if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			domain := strings.TrimSpace(reqData.Domain)
			if domain == "" {
				http.Error(w, "domain required", http.StatusBadRequest)
				return
			}

			s.mu.Lock()
			// Exact host membership (avoid substring false positives like api.com in evil-api.com)
			already := false
			for _, t := range strings.Split(s.TargetDomain, ",") {
				if normalizeHost(t) == normalizeHost(domain) {
					already = true
					break
				}
			}
			if !already {
				if s.TargetDomain == "" {
					s.TargetDomain = domain
				} else {
					s.TargetDomain = s.TargetDomain + "," + domain
				}
				if _, err := s.dbExec(`UPDATE sessions SET target = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, s.TargetDomain, s.SessionID); err != nil {
					s.mu.Unlock()
					http.Error(w, "Failed to update target: "+err.Error(), http.StatusInternalServerError)
					return
				}
				ensureServers(s.doc, s.TargetDomain)
			}
			s.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/sessions/switch", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "POST" {
			var reqData struct {
				ID int `json:"id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			if reqData.ID <= 0 {
				http.Error(w, "valid id required", http.StatusBadRequest)
				return
			}

			// Persist in-flight endpoints before replacing the active session.
			s.Flush()

			s.mu.Lock()
			var specJSON string
			var t string
			var ignore string
			err := s.dbQueryRow(`SELECT target, ignore_rules, spec_json FROM sessions WHERE id = ?`, reqData.ID).Scan(&t, &ignore, &specJSON)
			if err != nil {
				s.mu.Unlock()
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			doc, ok := s.loadAndMigrateSpec(reqData.ID, specJSON)
			if !ok {
				s.mu.Unlock()
				http.Error(w, "Failed to load session spec", http.StatusInternalServerError)
				return
			}
			s.doc = doc
			s.SessionID = reqData.ID
			s.TargetDomain = t
			s.IgnoreRules = ignore
			s.Discovered = s.loadDiscoveredDomainsLocked(reqData.ID)
			ensureServers(s.doc, s.TargetDomain)
			_, _ = s.dbExec(`UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, reqData.ID)
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": reqData.ID})
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/sessions/rename", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "POST" {
			var reqData struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			name := strings.TrimSpace(reqData.Name)
			if reqData.ID <= 0 || name == "" {
				http.Error(w, "ID and name required", http.StatusBadRequest)
				return
			}

			s.mu.Lock()
			result, err := s.dbExec(`UPDATE sessions SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, reqData.ID)
			s.mu.Unlock()

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if rows, _ := result.RowsAffected(); rows == 0 {
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/sessions/delete", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "POST" {
			var reqData struct {
				ID int `json:"id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			if reqData.ID <= 0 {
				http.Error(w, "valid id required", http.StatusBadRequest)
				return
			}

			s.mu.Lock()
			result, err := s.dbExec(`DELETE FROM sessions WHERE id = ?`, reqData.ID)
			if err != nil {
				s.mu.Unlock()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if rows, _ := result.RowsAffected(); rows == 0 {
				s.mu.Unlock()
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}

			// If we just deleted the active session, load whatever is left or create a fallback
			if s.SessionID == reqData.ID {
				var specJSON string
				var id int
				var target string
				var ignore string
				err := s.dbQueryRow(`SELECT id, target, ignore_rules, spec_json FROM sessions ORDER BY updated_at DESC LIMIT 1`).Scan(&id, &target, &ignore, &specJSON)
				if err == nil {
					if doc, ok := s.loadAndMigrateSpec(id, specJSON); ok {
						s.doc = doc
						s.SessionID = id
						s.TargetDomain = target
						s.IgnoreRules = ignore
						s.Discovered = s.loadDiscoveredDomainsLocked(id)
						ensureServers(s.doc, s.TargetDomain)
					}
				} else {
					// DB empty, fallback
					s.doc = newEmptySpec("example.com")
					s.TargetDomain = "example.com"
					s.IgnoreRules = ""
					data, _ := json.Marshal(s.doc)
					newID, insertErr := s.insertSession("Fallback", "example.com", "", string(data))
					if insertErr != nil {
						s.mu.Unlock()
						http.Error(w, "Failed to create fallback session: "+insertErr.Error(), http.StatusInternalServerError)
						return
					}
					s.SessionID = newID
					s.Discovered = make(map[string]bool)
				}
			}
			s.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/generate-sdk", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var reqData struct {
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		sdkDoc, excluded, err := specForSDK(s.doc)
		s.mu.Unlock()

		if err != nil {
			http.Error(w, "Failed to prepare SDK spec", http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(sdkDoc)
		if err != nil {
			http.Error(w, "Failed to serialize spec", http.StatusInternalServerError)
			return
		}

		if excluded > 0 {
			w.Header().Set("X-ShadowSchema-WebSocket-Excluded", fmt.Sprintf("%d", excluded))
		}

		language, err := normalizeSDKLanguage(reqData.Language)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		zipData, err := generateSDKZip(data, language)
		if err != nil {
			log.Printf("SDK Gen Error: %v", err)
			http.Error(w, "Failed to generate SDK: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s_sdk.zip\"", language))
		if _, err := w.Write(zipData); err != nil {
			log.Printf("Failed to write SDK zip response: %v", err)
		}
	})
}
