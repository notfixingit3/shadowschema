import type { AuthCredential } from "../client.js";

export interface ReplayScriptInput {
  path: string;
  method: string;
  operation: Record<string, unknown>;
  target: string;
  vault: AuthCredential[];
  specVault?: unknown;
}

export function vaultHeadersFromSpec(specVault: unknown): Record<string, string> {
  const headers: Record<string, string> = {};
  if (!Array.isArray(specVault)) {
    return headers;
  }

  for (const entry of specVault) {
    if (
      entry &&
      typeof entry === "object" &&
      "header_name" in entry &&
      "token_value" in entry &&
      typeof entry.header_name === "string" &&
      typeof entry.token_value === "string"
    ) {
      headers[entry.header_name] = entry.token_value;
    }
  }

  return headers;
}

export function resolveVaultHeaders(
  vault: AuthCredential[],
  specVault?: unknown,
): Record<string, string> {
  const fromSpec = vaultHeadersFromSpec(specVault);
  if (Object.keys(fromSpec).length > 0) {
    return fromSpec;
  }

  const headers: Record<string, string> = {};
  for (const credential of vault) {
    if (credential.header_name && credential.token_value) {
      headers[credential.header_name] = credential.token_value;
    }
  }
  return headers;
}

export function buildReplayUrl(path: string, target: string): string {
  const host = target.split(",")[0]?.trim() || "target-domain.com";
  const base = host.startsWith("http") ? host : `https://${host}`;
  return `${base.replace(/\/$/, "")}${path}`;
}

export function generatePythonReplayScript(input: ReplayScriptInput): string {
  const method = input.method.toUpperCase();
  const vaultHeaders = resolveVaultHeaders(input.vault, input.specVault);
  const headers = {
    "User-Agent": "ShadowSchema-Replay/1.0",
    ...vaultHeaders,
  };

  const url = buildReplayUrl(input.path, input.target);
  let script = `import requests\nimport json\n\nurl = "${url}"\n\nheaders = ${JSON.stringify(headers, null, 4)}\n\n`;

  let payloadKwarg = "";
  const payload = input.operation["x-last-payload"];
  if (["POST", "PUT", "PATCH"].includes(method) && payload !== undefined) {
    script += `payload = ${JSON.stringify(payload, null, 4)}\n\n`;
    payloadKwarg = ", json=payload";
  }

  const vaultNote =
    Object.keys(vaultHeaders).length > 0
      ? `# Auth headers auto-injected from ShadowSchema Auth Vault\n`
      : `# No Auth Vault credentials captured yet for this session\n`;

  script = vaultNote + script;
  script += `response = requests.request("${method}", url, headers=headers${payloadKwarg})\n\nprint(f"Status: {response.status_code}")\nprint(response.text)\n`;
  return script;
}