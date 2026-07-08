package spec

import (
	"encoding/json"

	"github.com/getkin/kin-openapi/openapi3"
)

// sensitiveOperationExtensions are stripped before SDK generation and other
// shareable artifacts so sample payloads / vault data never enter generated clients.
var sensitiveOperationExtensions = []string{
	"x-last-payload",
	"x-last-request-body",
	"x-payload-samples",
	"x-websocket-frames",
	"x-graphql-operations", // may embed variables + last responses
	"x-param-stats",
}

var sensitiveDocumentExtensions = []string{
	"x-shadowschema-vault",
}

// sanitizeDocForExport removes live secrets and bulky sample data from a cloned doc.
// Security schemes (names only) are preserved.
func sanitizeDocForExport(doc *openapi3.T) {
	if doc == nil {
		return
	}
	if doc.Extensions != nil {
		for _, key := range sensitiveDocumentExtensions {
			delete(doc.Extensions, key)
		}
	}
	if doc.Paths == nil {
		return
	}
	for _, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}
		for _, op := range []*openapi3.Operation{
			pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Delete,
			pathItem.Patch, pathItem.Head, pathItem.Options, pathItem.Trace,
		} {
			stripOperationSecrets(op)
		}
	}
}

func stripOperationSecrets(op *openapi3.Operation) {
	if op == nil || op.Extensions == nil {
		return
	}
	for _, key := range sensitiveOperationExtensions {
		delete(op.Extensions, key)
	}
	// Keep structural GraphQL flags without payloads.
	// x-graphql and x-graphql-operation-names are safe.
}

// redactCredentials returns a copy of credentials with token values blanked.
func redactCredentials(credentials []AuthCredential) []AuthCredential {
	out := make([]AuthCredential, len(credentials))
	for i, c := range credentials {
		out[i] = AuthCredential{
			HeaderName: c.HeaderName,
			Host:       c.Host,
			TokenValue: "",
			FirstSeen:  c.FirstSeen,
		}
		if c.TokenValue != "" {
			out[i].TokenValue = "[REDACTED]"
		}
	}
	return out
}

// cloneDocJSON deep-clones an OpenAPI document via JSON round-trip.
func cloneDocJSON(doc *openapi3.T) (*openapi3.T, error) {
	if doc == nil {
		return nil, nil
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var cloned openapi3.T
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}
