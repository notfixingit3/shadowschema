package spec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// SpecValidationResult is an agent-friendly OpenAPI health report.
type SpecValidationResult struct {
	Valid      bool                `json:"valid"`
	SessionID  int                 `json:"session_id"`
	CheckedAt  string              `json:"checked_at"`
	PathCount  int                 `json:"path_count"`
	OpCount    int                 `json:"operation_count"`
	Errors     []ValidationIssue   `json:"errors"`
	Warnings   []ValidationIssue   `json:"warnings"`
	Summary    string              `json:"summary"`
}

type ValidationIssue struct {
	Level   string `json:"level"` // error | warning
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Method  string `json:"method,omitempty"`
	Message string `json:"message"`
}

// ValidateSessionSpec validates the active or specified session's OpenAPI document.
func (s *SpecManager) ValidateSessionSpec(sessionID int, explicit bool) (SpecValidationResult, error) {
	view, err := s.sessionReadView(sessionID, explicit)
	if err != nil {
		return SpecValidationResult{}, err
	}
	result := validateOpenAPIDoc(view.Doc)
	result.SessionID = view.SessionID
	result.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

func validateOpenAPIDoc(doc *openapi3.T) SpecValidationResult {
	result := SpecValidationResult{
		Valid:    true,
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
	}
	if doc == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationIssue{
			Level: "error", Code: "empty_doc", Message: "OpenAPI document is nil",
		})
		result.Summary = "invalid: empty document"
		return result
	}

	// kin-openapi structural validation
	ctx := context.Background()
	if err := doc.Validate(ctx); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationIssue{
			Level:   "error",
			Code:    "openapi_validate",
			Message: err.Error(),
		})
	}

	if doc.OpenAPI == "" {
		result.Warnings = append(result.Warnings, ValidationIssue{
			Level: "warning", Code: "missing_openapi_version", Message: "openapi version field is empty",
		})
	}
	if len(doc.Servers) == 0 {
		result.Warnings = append(result.Warnings, ValidationIssue{
			Level: "warning", Code: "missing_servers", Message: "no servers defined; generators may guess the base URL",
		})
	}

	if doc.Paths != nil {
		result.PathCount = len(doc.Paths.Map())
		for path, item := range doc.Paths.Map() {
			if item == nil {
				continue
			}
			// Path template params must be declared.
			templateParams := pathTemplateNames(path)
			for _, method := range httpMethods {
				op := operationForMethod(item, method)
				if op == nil {
					continue
				}
				result.OpCount++
				declared := map[string]bool{}
				for _, p := range op.Parameters {
					if p != nil && p.Value != nil && p.Value.In == "path" {
						declared[p.Value.Name] = true
						if !p.Value.Required {
							result.Warnings = append(result.Warnings, ValidationIssue{
								Level: "warning", Code: "path_param_not_required",
								Path: path, Method: strings.ToUpper(method),
								Message: fmt.Sprintf("path parameter %q should be required", p.Value.Name),
							})
						}
					}
				}
				for _, name := range templateParams {
					if !declared[name] {
						result.Valid = false
						result.Errors = append(result.Errors, ValidationIssue{
							Level: "error", Code: "missing_path_parameter",
							Path: path, Method: strings.ToUpper(method),
							Message: fmt.Sprintf("path template {%s} has no matching path parameter", name),
						})
					}
				}
				if op.Responses == nil || len(op.Responses.Map()) == 0 {
					result.Warnings = append(result.Warnings, ValidationIssue{
						Level: "warning", Code: "no_responses",
						Path: path, Method: strings.ToUpper(method),
						Message: "operation has no responses",
					})
				}
			}
		}
	}

	if result.Valid && len(result.Errors) == 0 {
		result.Summary = fmt.Sprintf("valid: %d paths, %d operations, %d warnings",
			result.PathCount, result.OpCount, len(result.Warnings))
	} else {
		result.Valid = false
		result.Summary = fmt.Sprintf("invalid: %d errors, %d warnings (%d paths, %d operations)",
			len(result.Errors), len(result.Warnings), result.PathCount, result.OpCount)
	}
	return result
}

func pathTemplateNames(path string) []string {
	var names []string
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if len(part) >= 3 && part[0] == '{' && part[len(part)-1] == '}' {
			names = append(names, part[1:len(part)-1])
		}
	}
	return names
}
