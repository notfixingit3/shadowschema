package spec

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SessionDiffResult compares two session OpenAPI maps for agent-friendly recon.
type SessionDiffResult struct {
	FromSessionID int                 `json:"from_session_id"`
	ToSessionID   int                 `json:"to_session_id"`
	FromName      string              `json:"from_name,omitempty"`
	ToName        string              `json:"to_name,omitempty"`
	AddedPaths    []string            `json:"added_paths"`
	RemovedPaths  []string            `json:"removed_paths"`
	ChangedPaths  []PathChange        `json:"changed_paths"`
	Summary       SessionDiffSummary  `json:"summary"`
}

type PathChange struct {
	Path           string   `json:"path"`
	AddedMethods   []string `json:"added_methods,omitempty"`
	RemovedMethods []string `json:"removed_methods,omitempty"`
	// StatusCodesChanged is true when response status keys differ for a shared method.
	StatusCodesChanged bool `json:"status_codes_changed,omitempty"`
}

type SessionDiffSummary struct {
	AddedPathCount   int `json:"added_path_count"`
	RemovedPathCount int `json:"removed_path_count"`
	ChangedPathCount int `json:"changed_path_count"`
	FromEndpointCount int `json:"from_endpoint_count"`
	ToEndpointCount   int `json:"to_endpoint_count"`
}

// DiffSessions compares specs for two sessions (read-only; does not switch active).
func (s *SpecManager) DiffSessions(fromID, toID int) (SessionDiffResult, error) {
	if fromID <= 0 || toID <= 0 {
		return SessionDiffResult{}, fmt.Errorf("from and to session ids required")
	}

	fromView, err := s.sessionReadView(fromID, true)
	if err != nil {
		return SessionDiffResult{}, fmt.Errorf("from session: %w", err)
	}
	toView, err := s.sessionReadView(toID, true)
	if err != nil {
		return SessionDiffResult{}, fmt.Errorf("to session: %w", err)
	}

	result := diffDocs(fromView.Doc, toView.Doc)
	result.FromSessionID = fromID
	result.ToSessionID = toID
	result.FromName = fromView.Name
	result.ToName = toView.Name
	return result, nil
}

func diffDocs(from, to *openapi3.T) SessionDiffResult {
	fromMap := pathMethodMap(from)
	toMap := pathMethodMap(to)

	fromPaths := keysOf(fromMap)
	toPaths := keysOf(toMap)
	fromSet := stringSet(fromPaths)
	toSet := stringSet(toPaths)

	var added, removed []string
	for _, p := range toPaths {
		if !fromSet[p] {
			added = append(added, p)
		}
	}
	for _, p := range fromPaths {
		if !toSet[p] {
			removed = append(removed, p)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)

	var changed []PathChange
	for _, p := range toPaths {
		if !fromSet[p] {
			continue
		}
		fromMethods := fromMap[p]
		toMethods := toMap[p]
		ch := PathChange{Path: p}
		for m := range toMethods {
			if !fromMethods[m] {
				ch.AddedMethods = append(ch.AddedMethods, strings.ToUpper(m))
			}
		}
		for m := range fromMethods {
			if !toMethods[m] {
				ch.RemovedMethods = append(ch.RemovedMethods, strings.ToUpper(m))
			}
		}
		// Shared methods: compare response status keys.
		for m := range toMethods {
			if !fromMethods[m] {
				continue
			}
			if !sameStatusKeys(from, to, p, m) {
				ch.StatusCodesChanged = true
			}
		}
		sort.Strings(ch.AddedMethods)
		sort.Strings(ch.RemovedMethods)
		if len(ch.AddedMethods) > 0 || len(ch.RemovedMethods) > 0 || ch.StatusCodesChanged {
			changed = append(changed, ch)
		}
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Path < changed[j].Path })

	return SessionDiffResult{
		AddedPaths:   added,
		RemovedPaths: removed,
		ChangedPaths: changed,
		Summary: SessionDiffSummary{
			AddedPathCount:    len(added),
			RemovedPathCount:  len(removed),
			ChangedPathCount:  len(changed),
			FromEndpointCount: countOperations(from),
			ToEndpointCount:   countOperations(to),
		},
	}
}

func pathMethodMap(doc *openapi3.T) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if doc == nil || doc.Paths == nil {
		return out
	}
	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		methods := map[string]bool{}
		for _, m := range httpMethods {
			if operationForMethod(item, m) != nil {
				methods[m] = true
			}
		}
		if len(methods) > 0 {
			out[path] = methods
		}
	}
	return out
}

func sameStatusKeys(from, to *openapi3.T, path, method string) bool {
	fromOp := operationForMethod(from.Paths.Find(path), method)
	toOp := operationForMethod(to.Paths.Find(path), method)
	if fromOp == nil || toOp == nil {
		return fromOp == toOp
	}
	fromKeys := responseStatusKeys(fromOp)
	toKeys := responseStatusKeys(toOp)
	if len(fromKeys) != len(toKeys) {
		return false
	}
	for k := range fromKeys {
		if !toKeys[k] {
			return false
		}
	}
	return true
}

func responseStatusKeys(op *openapi3.Operation) map[string]bool {
	out := map[string]bool{}
	if op == nil || op.Responses == nil {
		return out
	}
	for code := range op.Responses.Map() {
		out[code] = true
	}
	return out
}

func countOperations(doc *openapi3.T) int {
	n := 0
	if doc == nil || doc.Paths == nil {
		return 0
	}
	for _, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for _, m := range httpMethods {
			if operationForMethod(item, m) != nil {
				n++
			}
		}
	}
	return n
}

func keysOf(m map[string]map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func stringSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, s := range items {
		out[s] = true
	}
	return out
}

func (s *SpecManager) mountDiffAndValidateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/sessions/diff", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var fromID, toID int
		var err error
		if r.Method == http.MethodGet {
			fromID, err = strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("from")))
			if err != nil || fromID <= 0 {
				http.Error(w, "from session id required", http.StatusBadRequest)
				return
			}
			toID, err = strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("to")))
			if err != nil || toID <= 0 {
				http.Error(w, "to session id required", http.StatusBadRequest)
				return
			}
		} else {
			var body struct {
				From int `json:"from"`
				To   int `json:"to"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			fromID, toID = body.From, body.To
		}

		result, err := s.DiffSessions(fromID, toID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "from session") || strings.Contains(err.Error(), "to session") {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("/validate-spec", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		sessionID, explicit, err := parseSessionIDQuery(r.URL.Query().Get("session_id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		result, err := s.ValidateSessionSpec(sessionID, explicit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if !result.Valid {
			w.WriteHeader(http.StatusOK) // always 200; valid flag in body
		}
		_ = json.NewEncoder(w).Encode(result)
	})
}
