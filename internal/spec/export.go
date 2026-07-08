package spec

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
)

func (s *SpecManager) buildExportDocument() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buildExportDocumentFrom(s.doc, s.SessionID, false)
}

func (s *SpecManager) buildExportDocumentFrom(doc *openapi3.T, sessionID int, includeSecrets bool) ([]byte, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	var cloned openapi3.T
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}

	s.enrichExportDocumentForSession(&cloned, sessionID, includeSecrets)
	return json.MarshalIndent(&cloned, "", "  ")
}

func (s *SpecManager) listVaultCredentials() ([]AuthCredential, error) {
	return s.listVaultCredentialsForSession(s.SessionID, "")
}

// listVaultCredentialsForSession returns vault entries for a session.
// When hostFilter is non-empty, returns credentials whose host matches the filter
// via hostMatchesTarget (so target example.com includes api.example.com) plus
// session-global (empty host) entries. More-specific hosts win over parents.
func (s *SpecManager) listVaultCredentialsForSession(sessionID int, hostFilter string) ([]AuthCredential, error) {
	hostFilter = normalizeHost(hostFilter)

	rows, err := s.dbQuery(
		`SELECT header_name, token_value, COALESCE(host, ''), first_seen FROM auth_vault WHERE session_id = ? ORDER BY first_seen DESC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []AuthCredential
	for rows.Next() {
		var ac AuthCredential
		if err := rows.Scan(&ac.HeaderName, &ac.TokenValue, &ac.Host, &ac.FirstSeen); err != nil {
			continue
		}
		ac.Host = normalizeHost(ac.Host)
		all = append(all, ac)
	}

	if hostFilter == "" {
		// Deduplicate by host+header (keep first = most recent).
		seen := make(map[string]bool)
		var out []AuthCredential
		for _, ac := range all {
			key := ac.Host + "\x00" + ac.HeaderName
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, ac)
		}
		return out, nil
	}

	// Rank matching credentials: exact host > longer subdomain match > empty global.
	type ranked struct {
		ac    AuthCredential
		score int
	}
	best := make(map[string]ranked) // header_name → best credential

	consider := func(ac AuthCredential, score int) {
		prev, ok := best[ac.HeaderName]
		if !ok || score > prev.score {
			best[ac.HeaderName] = ranked{ac: ac, score: score}
		}
	}

	for _, ac := range all {
		switch {
		case ac.Host == "":
			consider(ac, 1) // session-global fallback
		case ac.Host == hostFilter:
			consider(ac, 1000+len(ac.Host)) // exact match on filter
		case hostMatchesTarget(ac.Host, hostFilter):
			// Credential host is under filter (api.example.com under example.com)
			// or filter is under credential host — prefer longer (more specific) host.
			consider(ac, 100+len(ac.Host))
		case hostMatchesTarget(hostFilter, ac.Host):
			// Filter is more specific than credential host (filter api.x, cred x) —
			// credential may still apply as parent-scope token.
			consider(ac, 50+len(ac.Host))
		}
	}

	out := make([]AuthCredential, 0, len(best))
	for _, r := range best {
		out = append(out, r.ac)
	}
	return out, nil
}

// enrichExportDocumentForSession attaches security schemes from the vault.
// Token values are only included when includeSecrets is true (opt-in via
// ?include_secrets=1). Default exports never embed live credentials.
func (s *SpecManager) enrichExportDocumentForSession(doc *openapi3.T, sessionID int, includeSecrets bool) {
	credentials, err := s.listVaultCredentialsForSession(sessionID, "")
	if err != nil || len(credentials) == 0 {
		return
	}

	if doc.Components == nil {
		doc.Components = &openapi3.Components{}
	}
	if doc.Components.SecuritySchemes == nil {
		doc.Components.SecuritySchemes = openapi3.SecuritySchemes{}
	}

	for _, credential := range credentials {
		schemeName := securitySchemeName(credential.HeaderName)
		schemeType := "apiKey"
		in := "header"
		name := credential.HeaderName
		desc := "Captured auth header name from ShadowSchema Auth Vault (secret values omitted from default export)"

		if strings.EqualFold(credential.HeaderName, "Authorization") {
			// Prefer http bearer when values look like Bearer tokens.
			if strings.HasPrefix(strings.ToLower(credential.TokenValue), "bearer ") {
				doc.Components.SecuritySchemes[schemeName] = &openapi3.SecuritySchemeRef{
					Value: &openapi3.SecurityScheme{
						Type:         "http",
						Scheme:       "bearer",
						Description:  desc,
						BearerFormat: "JWT",
					},
				}
				continue
			}
		}
		if strings.EqualFold(credential.HeaderName, "Cookie") {
			in = "cookie"
			name = "session"
			desc = "Cookie-based auth observed via ShadowSchema Auth Vault (values omitted from default export)"
		}

		doc.Components.SecuritySchemes[schemeName] = &openapi3.SecuritySchemeRef{
			Value: &openapi3.SecurityScheme{
				Type:        schemeType,
				In:          in,
				Name:        name,
				Description: desc,
			},
		}
	}

	if !includeSecrets {
		return
	}

	if doc.Extensions == nil {
		doc.Extensions = make(map[string]interface{})
	}
	doc.Extensions["x-shadowschema-vault"] = credentials
}

func securitySchemeName(header string) string {
	var b strings.Builder
	for i, r := range header {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case i > 0:
			b.WriteRune('_')
		}
	}
	name := b.String()
	if name == "" {
		return "CapturedAuth"
	}
	return name
}
