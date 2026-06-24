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
	return s.buildExportDocumentFrom(s.doc, s.SessionID)
}

func (s *SpecManager) buildExportDocumentFrom(doc *openapi3.T, sessionID int) ([]byte, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	var cloned openapi3.T
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}

	s.enrichExportDocumentForSession(&cloned, sessionID)
	return json.MarshalIndent(&cloned, "", "  ")
}

func (s *SpecManager) listVaultCredentials() ([]AuthCredential, error) {
	return s.listVaultCredentialsForSession(s.SessionID)
}

func (s *SpecManager) listVaultCredentialsForSession(sessionID int) ([]AuthCredential, error) {
	rows, err := s.dbQuery(
		`SELECT header_name, token_value, first_seen FROM auth_vault WHERE session_id = ? ORDER BY first_seen DESC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var credentials []AuthCredential
	seen := make(map[string]bool)
	for rows.Next() {
		var ac AuthCredential
		if err := rows.Scan(&ac.HeaderName, &ac.TokenValue, &ac.FirstSeen); err != nil {
			continue
		}
		if seen[ac.HeaderName] {
			continue
		}
		seen[ac.HeaderName] = true
		credentials = append(credentials, ac)
	}
	return credentials, nil
}

func (s *SpecManager) enrichExportDocumentForSession(doc *openapi3.T, sessionID int) {
	credentials, err := s.listVaultCredentialsForSession(sessionID)
	if err != nil || len(credentials) == 0 {
		return
	}

	if doc.Extensions == nil {
		doc.Extensions = make(map[string]interface{})
	}
	doc.Extensions["x-shadowschema-vault"] = credentials

	if doc.Components == nil {
		doc.Components = &openapi3.Components{}
	}
	if doc.Components.SecuritySchemes == nil {
		doc.Components.SecuritySchemes = openapi3.SecuritySchemes{}
	}

	for _, credential := range credentials {
		schemeName := securitySchemeName(credential.HeaderName)
		doc.Components.SecuritySchemes[schemeName] = &openapi3.SecuritySchemeRef{
			Value: &openapi3.SecurityScheme{
				Type:        "apiKey",
				In:          "header",
				Name:        credential.HeaderName,
				Description: "Captured from intercepted traffic via ShadowSchema Auth Vault",
			},
		}
	}
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