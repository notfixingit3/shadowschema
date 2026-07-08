package spec

import (
	"net"
	"strings"
)

// normalizeHost strips port and lowercases a hostname.
func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	// Strip brackets from IPv6 literals before SplitHostPort.
	if strings.HasPrefix(host, "[") {
		if h, _, err := net.SplitHostPort(host); err == nil {
			return strings.ToLower(h)
		}
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(host)
}

// hostMatchesTarget returns true when host equals the target pattern or is a
// proper subdomain of it. Substring matching is intentionally avoided so
// "api.com" does not match "evil-api.com".
//
// Patterns:
//   - example.com        → example.com, api.example.com
//   - *.example.com      → api.example.com (not example.com)
//   - api.example.com    → api.example.com only (and its subdomains)
func hostMatchesTarget(host, pattern string) bool {
	host = normalizeHost(host)
	pattern = strings.TrimSpace(strings.ToLower(pattern))
	if host == "" || pattern == "" {
		return false
	}

	// Strip accidental scheme/path if a user pasted a URL as target.
	pattern = strings.TrimPrefix(pattern, "https://")
	pattern = strings.TrimPrefix(pattern, "http://")
	if i := strings.IndexAny(pattern, "/:"); i >= 0 {
		pattern = pattern[:i]
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}

	if strings.HasPrefix(pattern, "*.") {
		base := pattern[2:]
		if base == "" {
			return false
		}
		return strings.HasSuffix(host, "."+base)
	}

	return host == pattern || strings.HasSuffix(host, "."+pattern)
}

// primaryTargetHost returns the first comma-separated target host.
func primaryTargetHost(targetDomain string) string {
	parts := strings.Split(targetDomain, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "https://")
		p = strings.TrimPrefix(p, "http://")
		if i := strings.IndexAny(p, "/:"); i >= 0 {
			p = p[:i]
		}
		p = strings.TrimSpace(p)
		if p != "" {
			return p
		}
	}
	return ""
}

// serverURLForTarget builds an https:// base URL for OpenAPI servers.
func serverURLForTarget(targetDomain string) string {
	host := primaryTargetHost(targetDomain)
	if host == "" {
		return "https://example.com"
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/")
	}
	return "https://" + host
}
