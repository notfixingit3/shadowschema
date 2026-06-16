package router

import (
	"regexp"
	"strings"
)

var (
	uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
	yearRegex = regexp.MustCompile(`^(19|20)\d{2}$`)
	intRegex  = regexp.MustCompile(`^\d+$`)
)

// DeduplicatePath converts raw paths to templated paths
func DeduplicatePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if uuidRegex.MatchString(part) {
			parts[i] = "{uuid}"
		} else if yearRegex.MatchString(part) {
			parts[i] = "{year}"
		} else if intRegex.MatchString(part) {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}
