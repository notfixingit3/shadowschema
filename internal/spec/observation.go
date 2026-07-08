package spec

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	maxPayloadSamples     = 5
	requiredMinHits       = 3
	paramStatsExt         = "x-param-stats"
	hitCountExt           = "x-hit-count"
	statusHistogramExt    = "x-status-histogram"
	payloadSamplesExt     = "x-payload-samples"
)

var (
	intValueRegex  = regexp.MustCompile(`^-?\d+$`)
	boolValueRegex = regexp.MustCompile(`^(?i:true|false)$`)
	uuidValueRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// recordObservationStats updates hit counts, status histogram, multi-sample ring
// buffer, and query/header param presence stats for required inference.
func recordObservationStats(
	operation *openapi3.Operation,
	req *http.Request,
	statusCode int,
	responseBody, requestBody []byte,
	ignoreHeaders map[string]bool,
) {
	if operation.Extensions == nil {
		operation.Extensions = make(map[string]interface{})
	}

	hits := extensionInt(operation.Extensions[hitCountExt]) + 1
	operation.Extensions[hitCountExt] = hits

	hist := mapStringInt(operation.Extensions[statusHistogramExt])
	statusKey := strconv.Itoa(statusCode)
	hist[statusKey]++
	operation.Extensions[statusHistogramExt] = hist

	appendPayloadSample(operation, statusCode, responseBody, requestBody)

	stats := mapStringMap(operation.Extensions[paramStatsExt])
	seenKeys := map[string]bool{}

	// Query params present this hit.
	for key, values := range req.URL.Query() {
		statKey := "query:" + key
		seenKeys[statKey] = true
		entry := mapStringInterface(stats[statKey])
		entry["seen"] = interfaceInt(entry["seen"]) + 1
		if len(values) > 0 {
			entry["last_type"] = inferScalarType(values[0])
			if format := inferScalarFormat(values[0]); format != "" {
				entry["last_format"] = format
			}
		}
		stats[statKey] = entry
		// Ensure parameter exists and refine type.
		ensureQueryParamTyped(operation, key, values)
	}

	// Header params (non-ignored) present this hit.
	for key := range req.Header {
		canonical := http.CanonicalHeaderKey(key)
		if ignoreHeaders[canonical] {
			continue
		}
		statKey := "header:" + canonical
		seenKeys[statKey] = true
		entry := mapStringInterface(stats[statKey])
		entry["seen"] = interfaceInt(entry["seen"]) + 1
		stats[statKey] = entry
	}

	// Missed params: previously seen stats keys not present this hit.
	for statKey, raw := range stats {
		if seenKeys[statKey] {
			continue
		}
		entry := mapStringInterface(raw)
		entry["missed"] = interfaceInt(entry["missed"]) + 1
		stats[statKey] = entry
	}

	// Register headers that appear for the first time (seen above) without param.
	for key := range req.Header {
		canonical := http.CanonicalHeaderKey(key)
		if ignoreHeaders[canonical] {
			continue
		}
		ensureHeaderParam(operation, canonical)
	}

	operation.Extensions[paramStatsExt] = stats
	applyRequiredFromStats(operation, stats, hits)
}

func appendPayloadSample(operation *openapi3.Operation, statusCode int, responseBody, requestBody []byte) {
	sample := map[string]interface{}{
		"status":     statusCode,
		"captured_at": time.Now().UTC().Format(time.RFC3339),
	}
	if len(responseBody) > 0 {
		sample["response"] = decodeJSONPayload(responseBody)
	}
	if len(requestBody) > 0 {
		sample["request"] = decodeJSONPayload(requestBody)
	}

	var samples []interface{}
	if existing, ok := operation.Extensions[payloadSamplesExt].([]interface{}); ok {
		samples = existing
	}
	samples = append(samples, sample)
	if len(samples) > maxPayloadSamples {
		samples = samples[len(samples)-maxPayloadSamples:]
	}
	operation.Extensions[payloadSamplesExt] = samples
}

func applyRequiredFromStats(operation *openapi3.Operation, stats map[string]interface{}, hits int) {
	if hits < requiredMinHits || operation == nil {
		return
	}
	for _, p := range operation.Parameters {
		if p == nil || p.Value == nil {
			continue
		}
		// Path params are always required in OpenAPI.
		if p.Value.In == "path" {
			p.Value.Required = true
			continue
		}
		statKey := p.Value.In + ":" + p.Value.Name
		entry := mapStringInterface(stats[statKey])
		seen := interfaceInt(entry["seen"])
		missed := interfaceInt(entry["missed"])
		// Required when observed on every hit so far and we have enough samples.
		if seen >= requiredMinHits && missed == 0 && seen >= hits {
			p.Value.Required = true
		} else if missed > 0 {
			p.Value.Required = false
		}
	}
}

func ensureQueryParamTyped(operation *openapi3.Operation, name string, values []string) {
	var param *openapi3.Parameter
	for _, p := range operation.Parameters {
		if p.Value != nil && p.Value.Name == name && p.Value.In == "query" {
			param = p.Value
			break
		}
	}
	if param == nil {
		param = openapi3.NewQueryParameter(name)
		operation.AddParameter(param)
	}
	if len(values) == 0 {
		if param.Schema == nil {
			param.Schema = openapi3.NewSchemaRef("", openapi3.NewStringSchema())
		}
		return
	}
	param.Schema = openapi3.NewSchemaRef("", schemaForScalar(values[0]))
}

func ensureHeaderParam(operation *openapi3.Operation, name string) {
	for _, p := range operation.Parameters {
		if p.Value != nil && p.Value.Name == name && p.Value.In == "header" {
			return
		}
	}
	param := openapi3.NewHeaderParameter(name)
	param.Schema = openapi3.NewSchemaRef("", openapi3.NewStringSchema())
	operation.AddParameter(param)
}

func schemaForScalar(v string) *openapi3.Schema {
	switch inferScalarType(v) {
	case "integer":
		return openapi3.NewIntegerSchema()
	case "boolean":
		return openapi3.NewBoolSchema()
	case "number":
		return openapi3.NewFloat64Schema()
	default:
		s := openapi3.NewStringSchema()
		if f := inferScalarFormat(v); f != "" {
			s.Format = f
		}
		return s
	}
}

func inferScalarType(v string) string {
	if boolValueRegex.MatchString(v) {
		return "boolean"
	}
	if intValueRegex.MatchString(v) {
		return "integer"
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil && strings.Contains(v, ".") {
		return "number"
	}
	return "string"
}

func inferScalarFormat(v string) string {
	if uuidValueRegex.MatchString(v) {
		return "uuid"
	}
	return ""
}

func extensionInt(v interface{}) int {
	return interfaceInt(v)
}

func interfaceInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func mapStringInt(v interface{}) map[string]int {
	out := map[string]int{}
	switch m := v.(type) {
	case map[string]int:
		for k, val := range m {
			out[k] = val
		}
	case map[string]interface{}:
		for k, val := range m {
			out[k] = interfaceInt(val)
		}
	}
	return out
}

func mapStringMap(v interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	if m, ok := v.(map[string]interface{}); ok {
		for k, val := range m {
			out[k] = val
		}
	}
	return out
}

func mapStringInterface(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		out := make(map[string]interface{}, len(m)+2)
		for k, val := range m {
			out[k] = val
		}
		return out
	}
	return map[string]interface{}{}
}
