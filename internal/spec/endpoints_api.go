package spec

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

type HealthResponse struct {
	Status         string    `json:"status"`
	SessionID      int       `json:"session_id"`
	SessionName    string    `json:"session_name"`
	Target         string    `json:"target"`
	EndpointCount  int       `json:"endpoint_count"`
	ActiveSession  bool      `json:"active_session"`
	SessionUpdated time.Time `json:"session_updated_at"`
}

type EndpointIndexEntry struct {
	Path       string   `json:"path"`
	Methods    []string `json:"methods"`
	LastSeen   string   `json:"last_seen,omitempty"`
	HasPayload bool     `json:"has_payload"`
	WebSocket  bool     `json:"websocket"`
}

type EndpointIndexResponse struct {
	Count      int                  `json:"count"`
	SessionID  int                  `json:"session_id"`
	Endpoints  []EndpointIndexEntry `json:"endpoints"`
	PathPrefix string               `json:"path_prefix,omitempty"`
}

type sessionReadView struct {
	Doc        *openapi3.T
	SessionID  int
	Name       string
	Target     string
	UpdatedAt  time.Time
	IsActive   bool
}

var httpMethods = []string{"get", "post", "put", "delete", "patch", "head", "options", "trace"}

func parseSessionIDQuery(raw string) (int, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false, nil
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, false, fmt.Errorf("invalid session_id")
	}
	return id, true, nil
}

func (s *SpecManager) sessionReadView(sessionID int, explicit bool) (sessionReadView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !explicit {
		return sessionReadView{
			Doc:       s.doc,
			SessionID: s.SessionID,
			Name:      sessionNameLocked(s),
			Target:    s.TargetDomain,
			UpdatedAt: sessionUpdatedAtLocked(s),
			IsActive:  true,
		}, nil
	}

	var name, target, specJSON string
	var updatedAt time.Time
	err := s.dbQueryRow(
		`SELECT name, target, spec_json, updated_at FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&name, &target, &specJSON, &updatedAt)
	if err != nil {
		return sessionReadView{}, fmt.Errorf("session not found")
	}

	doc, ok := s.loadAndMigrateSpec(sessionID, specJSON)
	if !ok {
		return sessionReadView{}, fmt.Errorf("failed to load session spec")
	}

	return sessionReadView{
		Doc:       doc,
		SessionID: sessionID,
		Name:      name,
		Target:    target,
		UpdatedAt: updatedAt,
		IsActive:  sessionID == s.SessionID,
	}, nil
}

func sessionNameLocked(s *SpecManager) string {
	var name string
	_ = s.dbQueryRow(`SELECT name FROM sessions WHERE id = ?`, s.SessionID).Scan(&name)
	return name
}

func sessionUpdatedAtLocked(s *SpecManager) time.Time {
	var updatedAt time.Time
	_ = s.dbQueryRow(`SELECT updated_at FROM sessions WHERE id = ?`, s.SessionID).Scan(&updatedAt)
	return updatedAt
}

func (s *SpecManager) buildHealthResponse(view sessionReadView) HealthResponse {
	count := 0
	if view.Doc != nil && view.Doc.Paths != nil {
		count = len(view.Doc.Paths.Map())
	}
	return HealthResponse{
		Status:         "ok",
		SessionID:      view.SessionID,
		SessionName:    view.Name,
		Target:         view.Target,
		EndpointCount:  count,
		ActiveSession:  view.IsActive,
		SessionUpdated: view.UpdatedAt,
	}
}

func buildEndpointIndex(doc *openapi3.T, pathPrefix string) []EndpointIndexEntry {
	if doc == nil || doc.Paths == nil {
		return nil
	}

	entries := make([]EndpointIndexEntry, 0, len(doc.Paths.Map()))
	for path, pathItem := range doc.Paths.Map() {
		if pathPrefix != "" && !strings.HasPrefix(path, pathPrefix) {
			continue
		}
		if pathItem == nil {
			continue
		}

		entry := EndpointIndexEntry{Path: path}
		for _, method := range httpMethods {
			operation := operationForMethod(pathItem, method)
			if operation == nil {
				continue
			}
			entry.Methods = append(entry.Methods, strings.ToUpper(method))
			if operation.Extensions != nil {
				if _, ok := operation.Extensions["x-last-payload"]; ok {
					entry.HasPayload = true
				}
				if v, ok := operation.Extensions["x-websocket"].(bool); ok && v {
					entry.WebSocket = true
				}
				if seen, ok := operation.Extensions["x-last-seen"].(string); ok {
					if entry.LastSeen == "" || seen > entry.LastSeen {
						entry.LastSeen = seen
					}
				}
			}
		}
		if len(entry.Methods) > 0 {
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

func operationForMethod(pathItem *openapi3.PathItem, method string) *openapi3.Operation {
	switch method {
	case "get":
		return pathItem.Get
	case "post":
		return pathItem.Post
	case "put":
		return pathItem.Put
	case "delete":
		return pathItem.Delete
	case "patch":
		return pathItem.Patch
	case "head":
		return pathItem.Head
	case "options":
		return pathItem.Options
	case "trace":
		return pathItem.Trace
	default:
		return nil
	}
}

func endpointDetailFromDoc(doc *openapi3.T, path string) (map[string]interface{}, bool) {
	if doc == nil || doc.Paths == nil {
		return nil, false
	}
	pathItem := doc.Paths.Find(path)
	if pathItem == nil {
		return nil, false
	}

	operations := make(map[string]interface{})
	methods := make([]string, 0)
	for _, method := range httpMethods {
		operation := operationForMethod(pathItem, method)
		if operation == nil {
			continue
		}
		methods = append(methods, strings.ToUpper(method))
		raw, err := json.Marshal(operation)
		if err != nil {
			continue
		}
		var payload interface{}
		if err := json.Unmarshal(raw, &payload); err == nil {
			operations[method] = payload
		}
	}

	return map[string]interface{}{
		"path":       path,
		"methods":    methods,
		"operations": operations,
	}, true
}

func filterDocByPathPrefix(doc *openapi3.T, pathPrefix string) *openapi3.T {
	if doc == nil || pathPrefix == "" || doc.Paths == nil {
		return doc
	}

	filtered := *doc
	filtered.Paths = openapi3.NewPaths()
	for path, pathItem := range doc.Paths.Map() {
		if strings.HasPrefix(path, pathPrefix) {
			filtered.Paths.Set(path, pathItem)
		}
	}
	return &filtered
}

func (s *SpecManager) mountHealthAndEndpointRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
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

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.buildHealthResponse(view))
	})

	mux.HandleFunc("/endpoints", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
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

		pathPrefix := r.URL.Query().Get("path_prefix")
		entries := buildEndpointIndex(view.Doc, pathPrefix)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(EndpointIndexResponse{
			Count:      len(entries),
			SessionID:  view.SessionID,
			Endpoints:  entries,
			PathPrefix: pathPrefix,
		})
	})

	mux.HandleFunc("/endpoints/{path...}", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
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

		captured := strings.TrimPrefix(r.PathValue("path"), "/")
		if captured == "" {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}
		apiPath := "/" + captured

		detail, ok := endpointDetailFromDoc(view.Doc, apiPath)
		if !ok {
			http.Error(w, "endpoint not found", http.StatusNotFound)
			return
		}

		detail["session_id"] = view.SessionID
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(detail)
	})
}