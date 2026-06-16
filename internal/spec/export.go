package spec

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
)

func (s *SpecManager) buildExportDocument() ([]byte, error) {
	raw, err := json.Marshal(s.doc)
	if err != nil {
		return nil, err
	}

	var doc openapi3.T
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}

	s.enrichExportDocument(&doc)
	return json.MarshalIndent(&doc, "", "  ")
}

func (s *SpecManager) listVaultCredentials() ([]AuthCredential, error) {
	rows, err := s.db.Query(
		`SELECT header_name, token_value, first_seen FROM auth_vault WHERE session_id = ? ORDER BY first_seen DESC`,
		s.SessionID,
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

func (s *SpecManager) enrichExportDocument(doc *openapi3.T) {
	credentials, err := s.listVaultCredentials()
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