package spec

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"shadowschema/internal/parser"
)

func (s *SpecManager) AddWebSocketFrame(path, direction string, opcode byte, payload []byte, fragments int) {
	isControl := opcode >= 0x8 && opcode <= 0xA
	if len(payload) == 0 && !isControl {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.IgnoreRules != "" {
		if matched, _ := regexp.MatchString(s.IgnoreRules, path); matched {
			return
		}
	}

	pathItem := s.doc.Paths.Find(path)
	if pathItem == nil || pathItem.Get == nil {
		return
	}

	operation := pathItem.Get
	if operation.Extensions == nil || operation.Extensions["x-websocket"] != true {
		return
	}

	record := buildWebSocketFrameRecord(direction, opcode, payload, fragments)

	frames := websocketFramesFromExtensions(operation.Extensions)
	frames = append(frames, record)
	if len(frames) > maxWebSocketFrames {
		frames = frames[len(frames)-maxWebSocketFrames:]
	}
	stored := make([]interface{}, len(frames))
	for i, frame := range frames {
		stored[i] = frame
	}
	operation.Extensions["x-websocket-frames"] = stored
	updateWebSocketStats(operation, direction, opcode, fragments)
	s.inferWebSocketMessageSchema(operation, direction, opcode, record)

	s.saveState()
}

func buildWebSocketFrameRecord(direction string, opcode byte, payload []byte, fragments int) map[string]interface{} {
	record := map[string]interface{}{
		"direction":    direction,
		"opcode":       int(opcode),
		"opcode_name":  opcodeName(opcode),
		"captured_at":  time.Now().UTC().Format(time.RFC3339),
		"fragments":    fragments,
		"fragmented":   fragments > 1,
	}

	switch opcode {
	case 0x8:
		record["payload"] = decodeClosePayload(payload)
	case 0x9, 0xA:
		if len(payload) == 0 {
			record["payload"] = nil
		} else {
			record["payload"] = decodeWebSocketPayload(payload)
		}
	default:
		record["payload"] = decodeWebSocketPayload(payload)
	}

	return record
}

func decodeClosePayload(payload []byte) map[string]interface{} {
	result := map[string]interface{}{}
	if len(payload) >= 2 {
		code := int(payload[0])<<8 | int(payload[1])
		result["close_code"] = code
		if len(payload) > 2 {
			result["close_reason"] = string(payload[2:])
		}
	}
	if len(result) == 0 {
		result["close_code"] = 1005
		result["note"] = "No close code present in frame"
	}
	return result
}

func (s *SpecManager) inferWebSocketMessageSchema(operation *openapi3.Operation, direction string, opcode byte, record map[string]interface{}) {
	if opcode != 0x1 && opcode != 0x2 {
		return
	}

	payloadValue := record["payload"]
	switch raw := payloadValue.(type) {
	case map[string]interface{}:
		if encoding, ok := raw["encoding"].(string); ok && encoding == "base64" {
			if b64, ok := raw["base64"].(string); ok {
				if decoded, err := base64.StdEncoding.DecodeString(b64); err == nil {
					s.mergeDirectionWebSocketSchemaBytes(operation, direction, decoded)
				}
			}
			return
		}
		s.mergeDirectionWebSocketSchema(operation, direction, raw)
	case []interface{}:
		rawBytes, _ := json.Marshal(raw)
		s.mergeDirectionWebSocketSchemaBytes(operation, direction, rawBytes)
	case string:
		s.mergeDirectionWebSocketSchemaBytes(operation, direction, []byte(raw))
	}
}

func websocketSchemaKey(direction string) string {
	if direction == "in" {
		return "x-websocket-message-schema-in"
	}
	return "x-websocket-message-schema-out"
}

func updateWebSocketStats(operation *openapi3.Operation, direction string, opcode byte, fragments int) {
	stats := map[string]int{
		"total": 0, "data": 0, "control": 0, "in": 0, "out": 0, "fragmented": 0,
	}

	if existing, ok := operation.Extensions["x-websocket-stats"].(map[string]interface{}); ok {
		for key := range stats {
			stats[key] = extensionIntValue(existing[key])
		}
	}

	stats["total"]++
	if direction == "in" {
		stats["in"]++
	} else {
		stats["out"]++
	}
	if opcode >= 0x8 && opcode <= 0xA {
		stats["control"]++
	} else {
		stats["data"]++
	}
	if fragments > 1 {
		stats["fragmented"]++
	}

	operation.Extensions["x-websocket-stats"] = map[string]interface{}{
		"total":      stats["total"],
		"data":       stats["data"],
		"control":    stats["control"],
		"in":         stats["in"],
		"out":        stats["out"],
		"fragmented": stats["fragmented"],
	}
}

func (s *SpecManager) mergeDirectionWebSocketSchema(operation *openapi3.Operation, direction string, payload map[string]interface{}) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.mergeDirectionWebSocketSchemaBytes(operation, direction, raw)
}

func (s *SpecManager) mergeDirectionWebSocketSchemaBytes(operation *openapi3.Operation, direction string, raw []byte) {
	newSchema := parser.ParseResponseBody(raw)
	if newSchema == nil || newSchema.Value == nil {
		return
	}

	key := websocketSchemaKey(direction)
	existing := operation.Extensions[key]
	if existing == nil {
		operation.Extensions[key] = newSchema.Value
		return
	}

	existingRaw, err := json.Marshal(existing)
	if err != nil {
		operation.Extensions[key] = newSchema.Value
		return
	}

	existingSchema := parser.ParseResponseBody(existingRaw)
	merged := parser.MergeSchema(existingSchema, newSchema)
	operation.Extensions[key] = merged.Value
}

func websocketFramesFromExtensions(ext map[string]interface{}) []map[string]interface{} {
	rawFrames, ok := ext["x-websocket-frames"].([]interface{})
	if !ok {
		return nil
	}

	frames := make([]map[string]interface{}, 0, len(rawFrames))
	for _, item := range rawFrames {
		if frame, ok := item.(map[string]interface{}); ok {
			frames = append(frames, frame)
		}
	}
	return frames
}

func decodeWebSocketPayload(payload []byte) interface{} {
	var obj map[string]interface{}
	if err := json.Unmarshal(payload, &obj); err == nil {
		return obj
	}
	var arr []interface{}
	if err := json.Unmarshal(payload, &arr); err == nil {
		return arr
	}
	if len(payload) == 0 {
		return nil
	}
	return map[string]interface{}{
		"encoding": "base64",
		"base64":   base64.StdEncoding.EncodeToString(payload),
		"size":     len(payload),
	}
}

func extensionIntValue(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

func opcodeName(opcode byte) string {
	switch opcode {
	case 0x1:
		return "text"
	case 0x2:
		return "binary"
	case 0x8:
		return "close"
	case 0x9:
		return "ping"
	case 0xA:
		return "pong"
	default:
		return "unknown"
	}
}