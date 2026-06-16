package spec

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	_ "github.com/mattn/go-sqlite3"
	"shadowschema/internal/parser"
)

type SpecManager struct {
	mu           sync.Mutex
	doc          *openapi3.T
	db           *sql.DB
	SessionID    int
	TargetDomain string
	IgnoreRules  string
	Discovered   map[string]bool
}

type SessionMeta struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Target      string    `json:"target"`
	IgnoreRules string    `json:"ignore_rules"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewSpecManager(defaultTarget string) *SpecManager {
	db, err := sql.Open("sqlite3", "./shadowschema.db")
	if err != nil {
		log.Fatalf("Failed to open sqlite database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		name TEXT, 
		target TEXT, 
		spec_json TEXT, 
		ignore_rules TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatalf("Failed to create sessions table: %v", err)
	}

	// Safely add ignore_rules if updating existing db
	_, _ = db.Exec(`ALTER TABLE sessions ADD COLUMN ignore_rules TEXT DEFAULT ''`)

	sm := &SpecManager{db: db, Discovered: make(map[string]bool)}
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

	err := s.db.QueryRow(`SELECT id, target, ignore_rules, spec_json FROM sessions ORDER BY updated_at DESC LIMIT 1`).Scan(&id, &t, &ignore, &specJSON)
	if err == nil && specJSON != "" {
		doc, err := openapi3.NewLoader().LoadFromData([]byte(specJSON))
		if err == nil {
			s.doc = doc
			s.SessionID = id
			s.TargetDomain = t
			s.IgnoreRules = ignore
			s.Discovered = make(map[string]bool)
			return
		}
	}

	// Create new
	s.doc = &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{Title: "ShadowSchema Auto-Generated API", Version: "1.0.0"},
		Paths: openapi3.NewPaths(),
	}
	s.TargetDomain = target
	s.IgnoreRules = "\\.(png|jpg|jpeg|webp|gif|css|js|woff|woff2|ico)$"
	data, _ := json.Marshal(s.doc)
	res, err := s.db.Exec(`INSERT INTO sessions (name, target, ignore_rules, spec_json) VALUES (?, ?, ?, ?)`, "Initial Run", target, s.IgnoreRules, string(data))
	if err == nil {
		newID, _ := res.LastInsertId()
		s.SessionID = int(newID)
	}
}

func (s *SpecManager) GetTarget() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.TargetDomain
}

func (s *SpecManager) IsTarget(host string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	targets := strings.Split(s.TargetDomain, ",")
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t != "" && strings.Contains(host, t) {
			return true
		}
	}
	return false
}

func (s *SpecManager) AddDiscoveredDomain(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	host = strings.Split(host, ":")[0]
	if !s.Discovered[host] {
		s.Discovered[host] = true
	}
}

func (s *SpecManager) saveState() {
	data, err := json.Marshal(s.doc)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal spec for DB: %v", err)
		return
	}
	_, err = s.db.Exec(`UPDATE sessions SET spec_json = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(data), s.SessionID)
	if err != nil {
		log.Printf("[ERROR] Failed to save state to DB: %v", err)
	}
}

func (s *SpecManager) AddEndpoint(req *http.Request, path string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.IgnoreRules != "" {
		if matched, _ := regexp.MatchString(s.IgnoreRules, path); matched {
			return
		}
	}

	newSchema := parser.ParseResponseBody(body)

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

	// Capture last seen payload (convert to valid JSON object/string or base64)
	if operation.Extensions == nil {
		operation.Extensions = make(map[string]interface{})
	}
	
	if len(body) > 0 {
		var raw map[string]interface{}
		var rawArr []interface{}
		if err := json.Unmarshal(body, &raw); err == nil {
			operation.Extensions["x-last-payload"] = raw
		} else if err := json.Unmarshal(body, &rawArr); err == nil {
			operation.Extensions["x-last-payload"] = rawArr
		} else {
			operation.Extensions["x-last-payload"] = string(body)
		}
	}

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

	ignoreHeaders := map[string]bool{
		"Host": true, "Connection": true, "Accept-Encoding": true, "User-Agent": true,
		"Accept": true, "Accept-Language": true, "Sec-Fetch-Mode": true, "Sec-Fetch-Site": true,
		"Sec-Fetch-Dest": true, "Referer": true, "Origin": true, "Content-Length": true,
		"Content-Type": true, "X-Forwarded-For": true, "X-Forwarded-Proto": true,
		"Sec-Ch-Ua": true, "Sec-Ch-Ua-Mobile": true, "Sec-Ch-Ua-Platform": true,
	}
	for key := range req.Header {
		canonical := http.CanonicalHeaderKey(key)
		if !ignoreHeaders[canonical] {
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
	}

	resp := operation.Responses.Value("200")
	if resp == nil {
		mediaType := openapi3.NewMediaType()
		mediaType.Schema = newSchema
		content := openapi3.NewContentWithJSONSchema(newSchema.Value)
		respValue := openapi3.NewResponse().WithDescription("Auto-generated response").WithContent(content)
		operation.Responses.Set("200", &openapi3.ResponseRef{Value: respValue})
	} else {
		content := resp.Value.Content.Get("application/json")
		if content != nil && content.Schema != nil {
			content.Schema = parser.MergeSchema(content.Schema, newSchema)
		} else {
			if resp.Value.Content == nil {
				resp.Value.Content = openapi3.NewContent()
			}
			mediaType := openapi3.NewMediaType()
			mediaType.Schema = newSchema
			resp.Value.Content["application/json"] = mediaType
		}
	}

	s.saveState()
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

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func (s *SpecManager) StartExportServer(port string) {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/export-map", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		s.mu.Lock()
		data, err := json.MarshalIndent(s.doc, "", "  ")
		s.mu.Unlock()
		
		if err != nil {
			http.Error(w, "Failed to marshal spec", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "GET" {
			rows, err := s.db.Query(`SELECT id, name, target, ignore_rules, updated_at FROM sessions ORDER BY updated_at DESC`)
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

			s.mu.Lock()
			s.doc = &openapi3.T{
				OpenAPI: "3.0.0",
				Info: &openapi3.Info{Title: "ShadowSchema Auto-Generated API", Version: "1.0.0"},
				Paths: openapi3.NewPaths(),
			}
			s.TargetDomain = reqData.Target
			s.IgnoreRules = reqData.Ignore
			data, _ := json.Marshal(s.doc)
			res, _ := s.db.Exec(`INSERT INTO sessions (name, target, ignore_rules, spec_json) VALUES (?, ?, ?, ?)`, reqData.Name, reqData.Target, reqData.Ignore, string(data))
			newID, _ := res.LastInsertId()
			s.SessionID = int(newID)
			s.Discovered = make(map[string]bool)
			s.mu.Unlock()

			w.WriteHeader(http.StatusOK)
			return
		}
	})

	mux.HandleFunc("/discovered", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
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
		
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(keys)
	})

	mux.HandleFunc("/sessions/add-target", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
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

			s.mu.Lock()
			// Append to target
			if !strings.Contains(s.TargetDomain, reqData.Domain) {
				s.TargetDomain = s.TargetDomain + "," + reqData.Domain
				_, _ = s.db.Exec(`UPDATE sessions SET target = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, s.TargetDomain, s.SessionID)
			}
			s.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/sessions/switch", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
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

			s.mu.Lock()
			var specJSON string
			var t string
			var ignore string
			err := s.db.QueryRow(`SELECT target, ignore_rules, spec_json FROM sessions WHERE id = ?`, reqData.ID).Scan(&t, &ignore, &specJSON)
			if err == nil {
				doc, _ := openapi3.NewLoader().LoadFromData([]byte(specJSON))
				s.doc = doc
				s.SessionID = reqData.ID
				s.TargetDomain = t
				s.IgnoreRules = ignore
				s.Discovered = make(map[string]bool)
				_, _ = s.db.Exec(`UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, reqData.ID)
			}
			s.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/sessions/delete", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
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

			s.mu.Lock()
			_, _ = s.db.Exec(`DELETE FROM sessions WHERE id = ?`, reqData.ID)
			
			// If we just deleted the active session, load whatever is left or create a fallback
			if s.SessionID == reqData.ID {
				var specJSON string
				var id int
				var target string
				var ignore string
				err := s.db.QueryRow(`SELECT id, target, ignore_rules, spec_json FROM sessions ORDER BY updated_at DESC LIMIT 1`).Scan(&id, &target, &ignore, &specJSON)
				if err == nil {
					doc, _ := openapi3.NewLoader().LoadFromData([]byte(specJSON))
					s.doc = doc
					s.SessionID = id
					s.TargetDomain = target
					s.IgnoreRules = ignore
					s.Discovered = make(map[string]bool)
				} else {
					// DB empty, fallback
					s.doc = &openapi3.T{
						OpenAPI: "3.0.0",
						Info: &openapi3.Info{Title: "ShadowSchema Auto-Generated API", Version: "1.0.0"},
						Paths: openapi3.NewPaths(),
					}
					s.TargetDomain = "example.com"
					s.IgnoreRules = ""
					data, _ := json.Marshal(s.doc)
					res, _ := s.db.Exec(`INSERT INTO sessions (name, target, ignore_rules, spec_json) VALUES (?, ?, ?, ?)`, "Fallback", "example.com", "", string(data))
					newID, _ := res.LastInsertId()
					s.SessionID = int(newID)
					s.Discovered = make(map[string]bool)
				}
			}
			s.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}
	})

	fmt.Printf("[INFO] Export server running on %s\n", port)
	srv := &http.Server{
		Addr:              port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	_ = srv.ListenAndServe()
}
