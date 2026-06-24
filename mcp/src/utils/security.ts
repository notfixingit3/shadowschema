export const INFERRED_SCHEMA_NOTE =
  "Inferred from observed MITM traffic — not authoritative API documentation.";

export const LEGAL_NOTE =
  "Only use on APIs you own or have explicit permission to test. See ShadowSchema README Legal Disclaimer.";

export const VAULT_SECURITY_NOTE =
  "SENSITIVE: Returns captured auth credentials from your local ShadowSchema instance. Local use only — never commit, log, or share token values. " +
  LEGAL_NOTE;

export function redactVaultCredentials<T extends { header_name: string; token_value: string }>(
  credentials: T[],
): Array<T & { token_value: string }> {
  return credentials.map((credential) => ({
    ...credential,
    token_value: credential.token_value ? "[REDACTED]" : "",
  }));
}