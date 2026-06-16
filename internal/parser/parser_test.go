package parser

import (
	"testing"
)

func TestParseResponseBody(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		wantType string
	}{
		{
			name:     "valid json object",
			body:     []byte(`{"name": "Shadow", "id": 1}`),
			wantType: "object",
		},
		{
			name:     "valid json array",
			body:     []byte(`[1, 2, 3]`),
			wantType: "array",
		},
		{
			name:     "invalid json",
			body:     []byte(`invalid`),
			wantType: "string", // Falls back to string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseResponseBody(tt.body)
			if got == nil || got.Value == nil {
				t.Fatalf("ParseResponseBody returned nil")
			}
			if !got.Value.Type.Is(tt.wantType) {
				t.Errorf("ParseResponseBody() type = %v, want %v", got.Value.Type, tt.wantType)
			}
		})
	}
}

func TestMergeSchema(t *testing.T) {
	s1 := ParseResponseBody([]byte(`{"name": "Shadow"}`))
	s2 := ParseResponseBody([]byte(`{"id": 1}`))

	merged := MergeSchema(s1, s2)

	if merged == nil || merged.Value == nil {
		t.Fatalf("MergeSchema returned nil")
	}

	if !merged.Value.Type.Is("object") {
		t.Errorf("Expected merged to be object, got %v", merged.Value.Type)
	}

	if _, ok := merged.Value.Properties["name"]; !ok {
		t.Errorf("Merged schema missing 'name' property")
	}

	if _, ok := merged.Value.Properties["id"]; !ok {
		t.Errorf("Merged schema missing 'id' property")
	}
}
