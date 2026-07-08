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
		{
			name:     "empty body",
			body:     nil,
			wantType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseResponseBody(tt.body)
			if tt.wantType == "" {
				if got != nil {
					t.Fatalf("expected nil schema for empty body")
				}
				return
			}
			if got == nil || got.Value == nil {
				t.Fatalf("ParseResponseBody returned nil")
			}
			if !got.Value.Type.Is(tt.wantType) {
				t.Errorf("ParseResponseBody() type = %v, want %v", got.Value.Type, tt.wantType)
			}
		})
	}
}

func TestParseResponseBodyDetectsFormats(t *testing.T) {
	got := ParseResponseBody([]byte(`{
		"id": "123e4567-e89b-12d3-a456-426614174000",
		"email": "user@example.com",
		"created_at": "2024-01-02T15:04:05Z",
		"homepage": "https://example.com/path"
	}`))
	if got == nil || got.Value == nil {
		t.Fatal("nil schema")
	}
	props := got.Value.Properties
	if props["id"].Value.Format != "uuid" {
		t.Fatalf("expected uuid format, got %q", props["id"].Value.Format)
	}
	if props["email"].Value.Format != "email" {
		t.Fatalf("expected email format, got %q", props["email"].Value.Format)
	}
	if props["created_at"].Value.Format != "date-time" {
		t.Fatalf("expected date-time format, got %q", props["created_at"].Value.Format)
	}
	if props["homepage"].Value.Format != "uri" {
		t.Fatalf("expected uri format, got %q", props["homepage"].Value.Format)
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

func TestMergeSchemaNullableAndOneOf(t *testing.T) {
	s1 := ParseResponseBody([]byte(`{"value":"hello"}`))
	s2 := ParseResponseBody([]byte(`{"value":null}`))
	merged := MergeSchema(s1, s2)
	prop := merged.Value.Properties["value"]
	if prop == nil || prop.Value == nil || !prop.Value.Nullable {
		t.Fatalf("expected nullable string property, got %#v", prop)
	}

	s3 := ParseResponseBody([]byte(`{"value":1}`))
	merged2 := MergeSchema(s1, s3)
	prop2 := merged2.Value.Properties["value"]
	if prop2 == nil || prop2.Value == nil || len(prop2.Value.OneOf) < 2 {
		t.Fatalf("expected oneOf for type conflict, got %#v", prop2)
	}
}

func TestMergeSchemaMergesArrayItems(t *testing.T) {
	s1 := ParseResponseBody([]byte(`[{"id":1}]`))
	s2 := ParseResponseBody([]byte(`[{"id":2,"name":"x"}]`))
	merged := MergeSchema(s1, s2)
	if merged.Value.Items == nil || merged.Value.Items.Value == nil {
		t.Fatal("expected array items")
	}
	if _, ok := merged.Value.Items.Value.Properties["name"]; !ok {
		t.Fatalf("expected merged array item properties, got %#v", merged.Value.Items.Value.Properties)
	}
}
