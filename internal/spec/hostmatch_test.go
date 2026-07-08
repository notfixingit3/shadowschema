package spec

import "testing"

func TestHostMatchesTarget(t *testing.T) {
	tests := []struct {
		host, pattern string
		want          bool
	}{
		{"example.com", "example.com", true},
		{"api.example.com", "example.com", true},
		{"api.example.com:443", "example.com", true},
		{"evil-example.com", "example.com", false},
		{"notexample.com", "example.com", false},
		{"api.com.evil.com", "api.com", false},
		{"cdn.api.example.com", "api.example.com", true},
		{"api.example.com", "*.example.com", true},
		{"example.com", "*.example.com", false},
		{"unrelated.io", "example.com", false},
	}
	for _, tt := range tests {
		if got := hostMatchesTarget(tt.host, tt.pattern); got != tt.want {
			t.Errorf("hostMatchesTarget(%q, %q) = %v, want %v", tt.host, tt.pattern, got, tt.want)
		}
	}
}
