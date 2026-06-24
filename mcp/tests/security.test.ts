import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { redactVaultCredentials, VAULT_SECURITY_NOTE } from "../src/utils/security.js";

describe("security helpers", () => {
  it("redacts token values for safe logging", () => {
    const redacted = redactVaultCredentials([
      { header_name: "Authorization", token_value: "Bearer secret" },
    ]);
    assert.equal(redacted[0]?.token_value, "[REDACTED]");
    assert.equal(redacted[0]?.header_name, "Authorization");
  });

  it("vault security note mentions legal constraints", () => {
    assert.match(VAULT_SECURITY_NOTE, /permission/i);
    assert.match(VAULT_SECURITY_NOTE, /SENSITIVE/i);
  });
});