package spec

import (
	"encoding/json"

	"github.com/getkin/kin-openapi/openapi3"
)

func (s *SpecManager) loadAndMigrateSpec(sessionID int, specJSON string) (*openapi3.T, bool) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(specJSON))
	if err != nil {
		return nil, false
	}
	if migrateLegacyWebSocketSpecs(doc) {
		if data, err := json.Marshal(doc); err == nil {
			_, _ = s.dbExec(`UPDATE sessions SET spec_json = ? WHERE id = ?`, string(data), sessionID)
		}
	}
	return doc, true
}

const maxWebSocketFrames = 50

func isWebSocketOperation(op *openapi3.Operation) bool {
	if op == nil || op.Extensions == nil {
		return false
	}
	if v, ok := op.Extensions["x-websocket"].(bool); ok && v {
		return true
	}
	return false
}

func migrateLegacyWebSocketSpecs(doc *openapi3.T) bool {
	if doc == nil || doc.Paths == nil {
		return false
	}

	modified := false
	for _, pathItem := range doc.Paths.Map() {
		if pathItem == nil || pathItem.Trace == nil {
			continue
		}

		trace := pathItem.Trace
		if pathItem.Get == nil {
			pathItem.Get = trace
		} else {
			mergeWebSocketMetadata(pathItem.Get, trace)
		}

		if pathItem.Get.Extensions == nil {
			pathItem.Get.Extensions = make(map[string]interface{})
		}
		pathItem.Get.Extensions["x-websocket"] = true
		pathItem.Trace = nil
		modified = true
	}

	return modified
}

func mergeWebSocketMetadata(dst, src *openapi3.Operation) {
	if dst.Summary == "" {
		dst.Summary = src.Summary
	}
	if dst.Description == "" {
		dst.Description = src.Description
	}
	if dst.Extensions == nil {
		dst.Extensions = make(map[string]interface{})
	}
	for k, v := range src.Extensions {
		if _, exists := dst.Extensions[k]; !exists {
			dst.Extensions[k] = v
		}
	}
	for _, param := range src.Parameters {
		if param == nil || param.Value == nil {
			continue
		}
		exists := false
		for _, existing := range dst.Parameters {
			if existing.Value != nil &&
				existing.Value.Name == param.Value.Name &&
				existing.Value.In == param.Value.In {
				exists = true
				break
			}
		}
		if !exists {
			dst.AddParameter(param.Value)
		}
	}
}

func specForSDK(doc *openapi3.T) (*openapi3.T, int, error) {
	if doc == nil {
		return nil, 0, nil
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return nil, 0, err
	}

	var filtered openapi3.T
	if err := json.Unmarshal(data, &filtered); err != nil {
		return nil, 0, err
	}

	if filtered.Paths == nil {
		return &filtered, 0, nil
	}

	excluded := 0
	for path, pathItem := range filtered.Paths.Map() {
		if pathItem == nil {
			continue
		}

		if pathItem.Get != nil && isWebSocketOperation(pathItem.Get) {
			pathItem.Get = nil
			excluded++
		}
		if pathItem.Trace != nil {
			pathItem.Trace = nil
			excluded++
		}

		if pathItem.Get == nil && pathItem.Post == nil && pathItem.Put == nil &&
			pathItem.Delete == nil && pathItem.Patch == nil && pathItem.Head == nil &&
			pathItem.Options == nil && pathItem.Connect == nil {
			filtered.Paths.Delete(path)
		}
	}

	return &filtered, excluded, nil
}