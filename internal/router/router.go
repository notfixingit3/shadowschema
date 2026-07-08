package router

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	uuidRegex      = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	uuidHexRegex   = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
	ulidRegex      = regexp.MustCompile(`(?i)^[0-9A-HJKMNP-TV-Z]{26}$`)
	objectIDRegex  = regexp.MustCompile(`(?i)^[0-9a-f]{24}$`)
	sha1HexRegex   = regexp.MustCompile(`(?i)^[0-9a-f]{40}$`)
	sha256HexRegex = regexp.MustCompile(`(?i)^[0-9a-f]{64}$`)
	base64URLRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{22,}$`)
	yearRegex      = regexp.MustCompile(`^(19|20)\d{2}$`)
	snowflakeRegex = regexp.MustCompile(`^\d{16,20}$`)
	intRegex       = regexp.MustCompile(`^\d+$`)
)

// PathParam describes a templated path segment produced by DeduplicatePath.
type PathParam struct {
	Name   string // e.g. "id", "uuid", "id2"
	Schema string // openapi type hint: integer, string
	Format string // optional openapi format: uuid, etc.
}

// DeduplicatePath converts raw paths to templated paths with unique param names.
// Multiple integer segments become {id}, {id2}, {id3} (not repeated {id}).
func DeduplicatePath(path string) string {
	parts := strings.Split(path, "/")
	counts := make(map[string]int)
	for i, part := range parts {
		if part == "" {
			continue
		}
		base, _, _ := classifySegment(part)
		if base == "" {
			continue
		}
		counts[base]++
		name := uniqueParamName(base, counts[base])
		parts[i] = "{" + name + "}"
	}
	return strings.Join(parts, "/")
}

// uniqueParamName returns base for the first occurrence, base2/base3 for later ones.
func uniqueParamName(base string, n int) string {
	if n <= 1 {
		return base
	}
	return fmt.Sprintf("%s%d", base, n)
}

// PathParamsFromTemplate extracts path parameters from a templated path
// such as /users/{id}/orders/{id2} (all occurrences, unique names).
func PathParamsFromTemplate(path string) []PathParam {
	parts := strings.Split(path, "/")
	seen := make(map[string]bool)
	var params []PathParam
	for _, part := range parts {
		if len(part) < 3 || part[0] != '{' || part[len(part)-1] != '}' {
			continue
		}
		name := part[1 : len(part)-1]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		params = append(params, pathParamMeta(name))
	}
	return params
}

func pathParamMeta(name string) PathParam {
	base := stripTrailingDigits(name)
	if base == "" {
		base = name
	}
	switch base {
	case "id", "year", "snowflake":
		return PathParam{Name: name, Schema: "integer"}
	case "uuid":
		return PathParam{Name: name, Schema: "string", Format: "uuid"}
	default:
		return PathParam{Name: name, Schema: "string"}
	}
}

func stripTrailingDigits(s string) string {
	i := len(s)
	for i > 0 && unicode.IsDigit(rune(s[i-1])) {
		i--
	}
	return s[:i]
}

func classifySegment(part string) (name, schema, format string) {
	switch {
	case uuidRegex.MatchString(part):
		return "uuid", "string", "uuid"
	case uuidHexRegex.MatchString(part):
		return "uuid", "string", "uuid"
	case ulidRegex.MatchString(part):
		return "ulid", "string", ""
	case objectIDRegex.MatchString(part):
		return "objectId", "string", ""
	case sha256HexRegex.MatchString(part):
		return "hash", "string", ""
	case sha1HexRegex.MatchString(part):
		return "hash", "string", ""
	case yearRegex.MatchString(part):
		return "year", "integer", ""
	case snowflakeRegex.MatchString(part):
		return "snowflake", "integer", ""
	case intRegex.MatchString(part):
		return "id", "integer", ""
	case looksLikeBase64URLID(part):
		return "token", "string", ""
	default:
		return "", "", ""
	}
}

// looksLikeBase64URLID matches long base64url segments that are clearly opaque IDs,
// while avoiding normal words and short codes.
func looksLikeBase64URLID(part string) bool {
	if len(part) < 22 || len(part) > 128 {
		return false
	}
	if !base64URLRegex.MatchString(part) {
		return false
	}
	// Require mixed character classes so plain words are not templated.
	hasDigit := false
	hasLetter := false
	for _, r := range part {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		}
	}
	return hasDigit && hasLetter
}
