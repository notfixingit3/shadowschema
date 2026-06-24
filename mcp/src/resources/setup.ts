import type { Config } from "../config.js";

export function proxySetupMarkdown(config: Config): string {
  return `# ShadowSchema proxy setup

ShadowSchema intercepts HTTP/HTTPS traffic through a local MITM proxy and infers OpenAPI schemas from observed requests and responses.

## Service URLs

| Service | URL |
|---------|-----|
| Export API | ${config.exportUrl} |
| MITM proxy | ${config.proxyUrl} |
| Dashboard | http://localhost:8080 |

## Trust the MITM CA

Download the root CA:

\`\`\`bash
curl -fsS ${config.exportUrl}/ca-cert -o shadowschema-ca.crt
\`\`\`

Import \`shadowschema-ca.crt\` into your browser or OS trust store before browsing HTTPS targets.

## Browser proxy

| Setting | Value |
|---------|-------|
| HTTP proxy | 127.0.0.1:38080 |
| HTTPS proxy | 127.0.0.1:38080 |

## CLI environment variables

\`\`\`bash
export HTTP_PROXY=${config.proxyUrl}
export HTTPS_PROXY=${config.proxyUrl}
\`\`\`

## Workflow

1. Create a ShadowSchema session for your target host.
2. Route browser or CLI traffic through the MITM proxy.
3. Use the target application normally — endpoints appear in the live OpenAPI spec.
4. Read inferred schemas via ShadowSchema MCP tools.

> Schemas are **inferred from observed traffic**, not authoritative API documentation.
`;
}