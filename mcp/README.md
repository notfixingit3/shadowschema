# ShadowSchema MCP

Model Context Protocol server that exposes ShadowSchema's live API recon to coding agents.

## Status

Phase 3 — 17 tools + 2 resources, including Playwright crawl orchestration and Python replay export.

## Quick start

### 1. Start ShadowSchema

```bash
docker compose up -d
```

### 2. Run the MCP server

```bash
cd mcp
npm install
npm start
```

Or after building:

```bash
npm run build
node dist/index.js
```

### 3. Configure your agent

See [docs/agent-setup.md](docs/agent-setup.md) and copy a template from [examples/](examples/).

**Local dev config example** (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "shadowschema": {
      "command": "npm",
      "args": ["start"],
      "cwd": "/path/to/shadowschema/mcp",
      "env": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

## Tools

| Tool | Description |
|------|-------------|
| `shadowschema_health` | Verify export API connectivity |
| `shadowschema_list_sessions` | List recon sessions |
| `shadowschema_create_session` | Create a session for a target host |
| `shadowschema_switch_session` | Activate a saved session |
| `shadowschema_get_spec` | Full OpenAPI JSON/YAML (optional path filter) |
| `shadowschema_list_endpoints` | Compact endpoint index |
| `shadowschema_get_endpoint` | Single path detail with schemas and payloads |
| `shadowschema_get_vault` | Captured auth credentials |
| `shadowschema_list_discovered_domains` | Out-of-scope CONNECT hosts |
| `shadowschema_add_target_domain` | Expand target perimeter |
| `shadowschema_generate_sdk` | SDK zip (base64 or file path) |
| `shadowschema_get_ca_cert` | MITM root CA PEM |
| `shadowschema_get_setup_status` | Export API + proxy reachability and session metadata |
| `shadowschema_wait_for_endpoints` | Poll until endpoint coverage threshold is met |
| `shadowschema_spec_diff` | Return newly discovered paths since the last diff |
| `shadowschema_explore_target` | Headless crawl through MITM proxy to generate traffic |
| `shadowschema_export_replay_script` | Python `requests` replay script for one endpoint |

### Playwright setup (for `explore_target`)

```bash
cd mcp
npm install
npx playwright install chromium
```

## Resources

| URI | Description |
|-----|-------------|
| `shadowschema://spec/openapi.json` | Live inferred OpenAPI spec |
| `shadowschema://setup/proxy.md` | MITM proxy setup guide |

## Environment variables

| Variable | Default |
|----------|---------|
| `SHADOWSCHEMA_EXPORT_URL` | `http://localhost:38081` |
| `SHADOWSCHEMA_PROXY_URL` | `http://127.0.0.1:38080` |

## Development

```bash
npm test                  # unit tests (mocked HTTP)
npm run test:integration  # live checks against :38081
npm run build             # compile to dist/
```

## Docker sidecar (optional, dev/testing)

Not required for normal agent use. Builds a container with Playwright for isolated crawl/testing:

```bash
docker compose --profile mcp build mcp
docker compose --profile mcp run --rm -it mcp
```

Agents on your host should still connect via `npm start` (stdio MCP → `localhost:38081`).

## Docs

- [Agent setup guide](docs/agent-setup.md) — Grok Build, OpenCode, Cursor, Claude, VS Code
- [Agent recipes](docs/recipes.md) — copy-paste prompts for map/build, auth, WebSocket
- [Implementation plan](../todo.md)

> **Distribution:** Run from source (`npm start`) until manual testing is complete. npm publish is intentionally deferred.