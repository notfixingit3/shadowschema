package parser

import (
	"encoding/json"

	"github.com/getkin/kin-openapi/openapi3"
)

// ParseResponseBody takes raw JSON and infers an OpenAPI schema from it.
func ParseResponseBody(body []byte) *openapi3.SchemaRef {
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Not JSON, return a basic string schema
		return openapi3.NewSchemaRef("", openapi3.NewStringSchema())
	}

	return inferSchema(data)
}

func inferSchema(val interface{}) *openapi3.SchemaRef {
	if val == nil {
		return openapi3.NewSchemaRef("", openapi3.NewObjectSchema()) // Cannot infer type
	}

	switch v := val.(type) {
	case string:
		return openapi3.NewSchemaRef("", openapi3.NewStringSchema())
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
			// Infer from the first item
			items = inferSchema(v[0])
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

	// If types differ and neither is nil, we keep the existing type for simplicity
	// unless the existing type is empty object and the new type is something concrete.
	if !eVal.Type.Is(nVal.Type.Slice()[0]) {
		if eVal.Type.Is("object") && len(eVal.Properties) == 0 {
			return newSchema
		}
		return existing
	}

	if eVal.Type.Is("object") {
		if eVal.Properties == nil {
			eVal.Properties = make(openapi3.Schemas)
		}
		for key, nProp := range nVal.Properties {
			eProp, exists := eVal.Properties[key]
			if !exists {
				eVal.Properties[key] = nProp
			} else {
				eVal.Properties[key] = MergeSchema(eProp, nProp)
			}
		}
	} else if eVal.Type.Is("array") {
		eVal.Items = MergeSchema(eVal.Items, nVal.Items)
	}

	return existing
}
