package spec

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	_ "github.com/mattn/go-sqlite3"
	"shadowschema/internal/parser"
)

type SpecManager struct {
	mu  sync.Mutex
	doc *openapi3.T
	db  *sql.DB
}

func NewSpecManager() *SpecManager {
	db, err := sql.Open("sqlite3", "./shadowschema.db")
	if err != nil {
		log.Fatalf("Failed to open sqlite database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS state (id INTEGER PRIMARY KEY, spec_json TEXT)`)
	if err != nil {
		log.Fatalf("Failed to create state table: %v", err)
	}

	var doc *openapi3.T
	var specJSON string
	err = db.QueryRow(`SELECT spec_json FROM state WHERE id = 1`).Scan(&specJSON)
	if err == nil && specJSON != "" {
		doc, err = openapi3.NewLoader().LoadFromData([]byte(specJSON))
		if err != nil {
			log.Printf("[WARN] Failed to load spec from database: %v. Starting fresh.", err)
		}
	}

	if doc == nil {
		doc = &openapi3.T{
			OpenAPI: "3.0.0",
			Info: &openapi3.Info{
				Title:   "ShadowSchema Auto-Generated API",
				Version: "1.0.0",
			},
			Paths: openapi3.NewPaths(),
		}
	}

	return &SpecManager{
		doc: doc,
		db:  db,
	}
}

func (s *SpecManager) saveState() {
	data, err := json.Marshal(s.doc)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal spec for DB: %v", err)
		return
	}
	_, err = s.db.Exec(`INSERT OR REPLACE INTO state (id, spec_json) VALUES (1, ?)`, string(data))
	if err != nil {
		log.Printf("[ERROR] Failed to save state to DB: %v", err)
	}
}

func (s *SpecManager) AddEndpoint(method, path string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse body to schema
	newSchema := parser.ParseResponseBody(body)

	pathItem := s.doc.Paths.Find(path)
	if pathItem == nil {
		pathItem = &openapi3.PathItem{}
		s.doc.Paths.Set(path, pathItem)
	}

	var operation *openapi3.Operation
	switch method {
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

	// Use "200" as the default response for schema
	resp := operation.Responses.Value("200")
	if resp == nil {
		mediaType := openapi3.NewMediaType()
		mediaType.Schema = newSchema
		content := openapi3.NewContentWithJSONSchema(newSchema.Value)
		respValue := openapi3.NewResponse().WithDescription("Auto-generated response").WithContent(content)
		operation.Responses.Set("200", &openapi3.ResponseRef{Value: respValue})
	} else {
		// Merge schema
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

	// Persist the state
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

func (s *SpecManager) StartExportServer(port string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/export-map", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		data, err := json.MarshalIndent(s.doc, "", "  ")
		if err != nil {
			http.Error(w, "Failed to marshal spec", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		fmt.Println("[INFO] Exported OpenAPI spec via /export-map")
	})

	fmt.Printf("[INFO] Export server running on %s (try GET http://localhost%s/export-map)\n", port, port)
	http.ListenAndServe(port, mux)
}
