package spec

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
}

type SessionMeta struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Target    string    `json:"target"`
	UpdatedAt time.Time `json:"updated_at"`
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
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatalf("Failed to create sessions table: %v", err)
	}

	sm := &SpecManager{db: db}
	sm.LoadLatestOrCreate(defaultTarget)
	return sm
}

func (s *SpecManager) LoadLatestOrCreate(target string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var specJSON string
	var id int
	var t string

	err := s.db.QueryRow(`SELECT id, target, spec_json FROM sessions ORDER BY updated_at DESC LIMIT 1`).Scan(&id, &t, &specJSON)
	if err == nil && specJSON != "" {
		doc, err := openapi3.NewLoader().LoadFromData([]byte(specJSON))
		if err == nil {
			s.doc = doc
			s.SessionID = id
			s.TargetDomain = t
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
	data, _ := json.Marshal(s.doc)
	res, err := s.db.Exec(`INSERT INTO sessions (name, target, spec_json) VALUES (?, ?, ?)`, "Initial Run", target, string(data))
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

	return os.WriteFile(filename, data, 0644)
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
		w.Write(data)
	})

	mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "GET" {
			rows, err := s.db.Query(`SELECT id, name, target, updated_at FROM sessions ORDER BY updated_at DESC`)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var sessions []SessionMeta
			for rows.Next() {
				var sm SessionMeta
				rows.Scan(&sm.ID, &sm.Name, &sm.Target, &sm.UpdatedAt)
				sessions = append(sessions, sm)
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sessions)
			return
		}

		if r.Method == "POST" {
			var reqData struct {
				Name   string `json:"name"`
				Target string `json:"target"`
			}
			json.NewDecoder(r.Body).Decode(&reqData)
			
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
			data, _ := json.Marshal(s.doc)
			res, _ := s.db.Exec(`INSERT INTO sessions (name, target, spec_json) VALUES (?, ?, ?)`, reqData.Name, reqData.Target, string(data))
			newID, _ := res.LastInsertId()
			s.SessionID = int(newID)
			s.mu.Unlock()

			w.WriteHeader(http.StatusOK)
			return
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
			json.NewDecoder(r.Body).Decode(&reqData)

			s.mu.Lock()
			var specJSON string
			var t string
			err := s.db.QueryRow(`SELECT target, spec_json FROM sessions WHERE id = ?`, reqData.ID).Scan(&t, &specJSON)
			if err == nil {
				doc, _ := openapi3.NewLoader().LoadFromData([]byte(specJSON))
				s.doc = doc
				s.SessionID = reqData.ID
				s.TargetDomain = t
				s.db.Exec(`UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, reqData.ID)
			}
			s.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}
	})

	fmt.Printf("[INFO] Export server running on %s\n", port)
	http.ListenAndServe(port, mux)
}
