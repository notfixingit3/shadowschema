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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeduplicatePath(tt.input); got != tt.expected {
				t.Errorf("DeduplicatePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
