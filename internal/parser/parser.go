package parser

import (
	"encoding/json"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	uuidFormatRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	dateOnlyRegex   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// ParseResponseBody takes raw JSON and infers an OpenAPI schema from it.
func ParseResponseBody(body []byte) *openapi3.SchemaRef {
	if len(body) == 0 {
		return nil
	}
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Not JSON, return a basic string schema
		return openapi3.NewSchemaRef("", openapi3.NewStringSchema())
	}

	return inferSchema(data)
}

// ParseRequestBody is an alias for ParseResponseBody (same inference rules).
func ParseRequestBody(body []byte) *openapi3.SchemaRef {
	return ParseResponseBody(body)
}

func inferSchema(val interface{}) *openapi3.SchemaRef {
	if val == nil {
		schema := openapi3.NewSchema()
		schema.Nullable = true
		return openapi3.NewSchemaRef("", schema)
	}

	switch v := val.(type) {
	case string:
		schema := openapi3.NewStringSchema()
		if format := detectStringFormat(v); format != "" {
			schema.Format = format
		}
		return openapi3.NewSchemaRef("", schema)
	case float64:
		// JSON numbers are unmarshaled to float64
		// Heuristically check if it's an integer
		if v == float64(int64(v)) {
			return openapi3.NewSchemaRef("", openapi3.NewIntegerSchema())
		}
		return openapi3.NewSchemaRef("", openapi3.NewFloat64Schema())
	case bool:
		return openapi3.NewSchemaRef("", openapi3.NewBoolSchema())
	case []interface{}:
		items := openapi3.NewSchemaRef("", openapi3.NewObjectSchema()) // default items
		if len(v) > 0 {
			items = inferSchema(v[0])
			for i := 1; i < len(v); i++ {
				items = MergeSchema(items, inferSchema(v[i]))
			}
		}
		schema := openapi3.NewArraySchema()
		schema.Items = items
		return openapi3.NewSchemaRef("", schema)
	case map[string]interface{}:
		schema := openapi3.NewObjectSchema()
		for key, value := range v {
			schema.Properties[key] = inferSchema(value)
		}
		return openapi3.NewSchemaRef("", schema)
	default:
		return openapi3.NewSchemaRef("", openapi3.NewObjectSchema())
	}
}

func detectStringFormat(s string) string {
	if uuidFormatRegex.MatchString(s) {
		return "uuid"
	}
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return "date-time"
	}
	if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return "date-time"
	}
	if dateOnlyRegex.MatchString(s) {
		if _, err := time.Parse("2006-01-02", s); err == nil {
			return "date"
		}
	}
	if strings.Contains(s, "@") {
		if _, err := mail.ParseAddress(s); err == nil {
			return "email"
		}
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if u, err := url.ParseRequestURI(s); err == nil && u.Host != "" {
			return "uri"
		}
	}
	return ""
}

func primaryType(schema *openapi3.Schema) string {
	if schema == nil || schema.Type == nil || len(schema.Type.Slice()) == 0 {
		return ""
	}
	return schema.Type.Slice()[0]
}

// MergeSchema merges a newly discovered schema into an existing schema, enabling schema evolution.
func MergeSchema(existing, newSchema *openapi3.SchemaRef) *openapi3.SchemaRef {
	if existing == nil || existing.Value == nil {
		return newSchema
	}
	if newSchema == nil || newSchema.Value == nil {
		return existing
	}

	eVal := existing.Value
	nVal := newSchema.Value

	eType := primaryType(eVal)
	nType := primaryType(nVal)

	// Null observations mark the field as nullable.
	if nType == "" && nVal.Nullable {
		eVal.Nullable = true
		return existing
	}
	if eType == "" && eVal.Nullable {
		nVal.Nullable = true
		return newSchema
	}

	// Type conflicts: prefer oneOf when both types are concrete and differ.
	if eType != "" && nType != "" && eType != nType {
		// Empty object yields to a concrete type.
		if eType == "object" && len(eVal.Properties) == 0 {
			return newSchema
		}
		if nType == "object" && len(nVal.Properties) == 0 {
			return existing
		}
		// integer vs number → number
		if (eType == "integer" && nType == "number") || (eType == "number" && nType == "integer") {
			eVal.Type = &openapi3.Types{"number"}
			return existing
		}
		return mergeAsOneOf(existing, newSchema)
	}

	if eType == "object" || (eType == "" && len(eVal.Properties) > 0) {
		if eVal.Properties == nil {
			eVal.Properties = make(openapi3.Schemas)
		}
		for key, nProp := range nVal.Properties {
			eProp, exists := eVal.Properties[key]
			if !exists {
				// Property seen later → may be optional; leave required unset
				eVal.Properties[key] = nProp
			} else {
				eVal.Properties[key] = MergeSchema(eProp, nProp)
			}
		}
		// Properties present in existing but missing from new sample stay optional.
	} else if eType == "array" {
		eVal.Items = MergeSchema(eVal.Items, nVal.Items)
	} else if eType == "string" {
		// Prefer a more specific format when one side has it.
		if eVal.Format == "" && nVal.Format != "" {
			eVal.Format = nVal.Format
		}
	}

	if nVal.Nullable {
		eVal.Nullable = true
	}

	return existing
}

func mergeAsOneOf(existing, newSchema *openapi3.SchemaRef) *openapi3.SchemaRef {
	// If existing already has oneOf, append when type is new.
	if existing.Value != nil && len(existing.Value.OneOf) > 0 {
		for _, branch := range existing.Value.OneOf {
			if schemasCompatible(branch, newSchema) {
				return existing
			}
		}
		existing.Value.OneOf = append(existing.Value.OneOf, newSchema)
		return existing
	}

	wrapper := openapi3.NewSchema()
	wrapper.OneOf = openapi3.SchemaRefs{existing, newSchema}
	return openapi3.NewSchemaRef("", wrapper)
}

func schemasCompatible(a, b *openapi3.SchemaRef) bool {
	if a == nil || b == nil || a.Value == nil || b.Value == nil {
		return false
	}
	return primaryType(a.Value) == primaryType(b.Value) && primaryType(a.Value) != ""
}
