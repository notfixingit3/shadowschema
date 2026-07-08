package spec

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	graphqlPathHint = regexp.MustCompile(`(?i)(^|/)(graphql|gql)(/|$|\?)`)
	// operation keyword + optional name, then selection set start
	gqlOpHeader = regexp.MustCompile(`(?is)\b(query|mutation|subscription)\s*([A-Za-z_][A-Za-z0-9_]*)?\s*(\([^)]*\))?\s*\{`)
	// first field after opening brace of operation (rough)
	gqlFirstField = regexp.MustCompile(`(?s)\{\s*([A-Za-z_][A-Za-z0-9_]*)`)
)

// mergeGraphQLMetadata inspects request (and optionally response) bodies for GraphQL
// and records operations under x-graphql-operations on the OpenAPI operation.
func mergeGraphQLMetadata(operation *openapi3.Operation, path string, requestBody, responseBody []byte) {
	if operation == nil {
		return
	}
	if !looksLikeGraphQL(path, requestBody) {
		return
	}

	ops := parseGraphQLOperations(requestBody)
	if len(ops) == 0 {
		return
	}

	if operation.Extensions == nil {
		operation.Extensions = make(map[string]interface{})
	}
	operation.Extensions["x-graphql"] = true

	existing := graphQLOpsMap(operation.Extensions["x-graphql-operations"])
	now := time.Now().UTC().Format(time.RFC3339)

	for _, op := range ops {
		key := op.Key
		entry, ok := existing[key].(map[string]interface{})
		if !ok {
			entry = map[string]interface{}{}
		}
		entry["name"] = op.Name
		entry["type"] = op.Type
		if len(op.Fields) > 0 {
			entry["fields"] = op.Fields
		}
		if op.OperationName != "" {
			entry["operationName"] = op.OperationName
		}
		if op.Variables != nil {
			entry["x-last-variables"] = op.Variables
		}
		entry["x-last-seen"] = now
		if len(responseBody) > 0 {
			if payload := decodeJSONPayload(responseBody); payload != nil {
				entry["x-last-response"] = payload
			}
		}
		existing[key] = entry
	}

	operation.Extensions["x-graphql-operations"] = existing

	// Keep a compact index for agents.
	names := make([]string, 0, len(existing))
	for k := range existing {
		names = append(names, k)
	}
	operation.Extensions["x-graphql-operation-names"] = names
}

type graphQLOpParsed struct {
	Key           string
	Name          string
	Type          string
	OperationName string
	Fields        []string
	Variables     interface{}
}

func looksLikeGraphQL(path string, requestBody []byte) bool {
	if graphqlPathHint.MatchString(path) {
		return true
	}
	if len(requestBody) == 0 {
		return false
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(requestBody, &envelope); err != nil {
		return false
	}
	q, _ := envelope["query"].(string)
	return strings.Contains(q, "{") || envelope["operationName"] != nil
}

func parseGraphQLOperations(requestBody []byte) []graphQLOpParsed {
	if len(requestBody) == 0 {
		return nil
	}

	// Single operation object
	var single map[string]interface{}
	if err := json.Unmarshal(requestBody, &single); err == nil {
		if _, hasQuery := single["query"]; hasQuery {
			return []graphQLOpParsed{parseGraphQLEnvelope(single)}
		}
	}

	// Batch: array of operations
	var batch []map[string]interface{}
	if err := json.Unmarshal(requestBody, &batch); err == nil && len(batch) > 0 {
		out := make([]graphQLOpParsed, 0, len(batch))
		for _, item := range batch {
			if _, hasQuery := item["query"]; hasQuery {
				out = append(out, parseGraphQLEnvelope(item))
			}
		}
		return out
	}

	return nil
}

func parseGraphQLEnvelope(envelope map[string]interface{}) graphQLOpParsed {
	query, _ := envelope["query"].(string)
	opName, _ := envelope["operationName"].(string)
	opType, name, fields := inspectGraphQLQuery(query)

	if opName == "" {
		opName = name
	}
	if opName == "" {
		opName = "anonymous"
	}
	if opType == "" {
		opType = "query"
	}
	if name == "" {
		name = opName
	}

	key := opType + ":" + opName
	var variables interface{}
	if v, ok := envelope["variables"]; ok {
		variables = v
	}

	return graphQLOpParsed{
		Key:           key,
		Name:          name,
		Type:          opType,
		OperationName: opName,
		Fields:        fields,
		Variables:     variables,
	}
}

func inspectGraphQLQuery(query string) (opType, name string, fields []string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", "", nil
	}

	if m := gqlOpHeader.FindStringSubmatch(query); len(m) >= 3 {
		opType = strings.ToLower(m[1])
		name = m[2]
	} else {
		// Shorthand query: { user { id } }
		opType = "query"
	}

	// Prefer selection fields after the operation header when present.
	searchIn := query
	if m := gqlOpHeader.FindStringIndex(query); m != nil {
		searchIn = query[m[0]:]
	}
	if fm := gqlFirstField.FindStringSubmatch(searchIn); len(fm) >= 2 {
		fields = []string{fm[1]}
		if name == "" {
			name = fm[1]
		}
	}

	return opType, name, fields
}

func graphQLOpsMap(raw interface{}) map[string]interface{} {
	if raw == nil {
		return map[string]interface{}{}
	}
	if m, ok := raw.(map[string]interface{}); ok {
		return m
	}
	// After JSON round-trip through kin-openapi, may already be map[string]interface{}
	b, err := json.Marshal(raw)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}
