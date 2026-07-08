package router

import (
	"testing"
)

func TestDeduplicatePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "uuid deduplication",
			input:    "/api/v1/users/123e4567-e89b-12d3-a456-426614174000/profile",
			expected: "/api/v1/users/{uuid}/profile",
		},
		{
			name:     "integer deduplication",
			input:    "/drivers/44/telemetry",
			expected: "/drivers/{id}/telemetry",
		},
		{
			name:     "year deduplication",
			input:    "/api/v1/races/2026/telemetry",
			expected: "/api/v1/races/{year}/telemetry",
		},
		{
			name:     "multiple variables",
			input:    "/api/v1/races/2026/drivers/44",
			expected: "/api/v1/races/{year}/drivers/{id}",
		},
		{
			name:     "no variables",
			input:    "/api/v1/status",
			expected: "/api/v1/status",
		},
		{
			name:     "empty path",
			input:    "/",
			expected: "/",
		},
		{
			name:     "ulid",
			input:    "/orders/01ARZ3NDEKTSV4RRFFQ69G5FAV",
			expected: "/orders/{ulid}",
		},
		{
			name:     "mongo object id",
			input:    "/docs/507f1f77bcf86cd799439011",
			expected: "/docs/{objectId}",
		},
		{
			name:     "uuid without dashes",
			input:    "/items/123e4567e89b12d3a456426614174000",
			expected: "/items/{uuid}",
		},
		{
			name:     "snowflake id",
			input:    "/messages/123456789012345678",
			expected: "/messages/{snowflake}",
		},
		{
			name:     "does not match parent domain words",
			input:    "/api/v2/users",
			expected: "/api/v2/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeduplicatePath(tt.input); got != tt.expected {
				t.Errorf("DeduplicatePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPathParamsFromTemplate(t *testing.T) {
	params := PathParamsFromTemplate("/api/users/{id}/orders/{uuid}")
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	if params[0].Name != "id" || params[0].Schema != "integer" {
		t.Fatalf("unexpected first param: %#v", params[0])
	}
	if params[1].Name != "uuid" || params[1].Format != "uuid" {
		t.Fatalf("unexpected second param: %#v", params[1])
	}
}
