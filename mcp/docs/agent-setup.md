# ShadowSchema MCP — Agent Setup Guide

Connect coding agents to ShadowSchema so they can discover undocumented APIs, read inferred schemas, and build apps against live traffic — without manual dashboard exports.

**Example configs:** [`mcp/examples/`](../examples/)  
**Implementation status:** Phase 1 MCP server is available in [`mcp/`](../). Build with `cd mcp && npm install && npm run build`, then launch via `node dist/index.js` in MCP configs.

---

## Contents

- [How it works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Launch command](#launch-command)
- [Grok Build](#grok-build)
- [OpenCode](#opencode)
- [Cursor](#cursor)
- [Claude Code](#claude-code)
- [Claude Desktop](#claude-desktop)
- [VS Code (Copilot)](#vs-code-copilot)
- [Antigravity](#antigravity)
- [Portable project config](#portable-project-config)
- [Companion MCPs](#companion-mcps)
- [Agent recipes](#agent-recipes)
- [Example workflows](#example-workflows)
- [Troubleshooting](#troubleshooting)
- [Security](#security)

---

## How it works

```
Coding Agent  ←──MCP tools──→  shadowschema-mcp  ←──HTTP──→  Export API (:38081)
       │
       └── Browser / Playwright MCP ──→ MITM Proxy (:38080) ──→ Target API
```

ShadowSchema passively maps traffic into OpenAPI. The MCP server wraps the existing export API (`/export-map`, `/sessions`, `/vault`, etc.) as agent tools. Traffic generation still happens through a browser or HTTP client routed via the MITM proxy.

---

## Prerequisites

### 1. Start ShadowSchema

From the repo root:

```bash
docker compose up -d
```

| Service | URL |
|---------|-----|
| Dashboard | http://localhost:8080 |
| MITM proxy | `127.0.0.1:38080` |
| Export API | http://localhost:38081 |

### 2. Verify the export API

```bash
curl -s http://localhost:38081/export-map | head
```

You should see JSON with `"openapi": "3.0.0"`.

### 3. Trust the MITM CA (for HTTPS targets)

Download the CA from the dashboard (**🔒 CA Cert**) or:

```bash
curl -fsS http://localhost:38081/ca-cert -o shadowschema-ca.crt
```

Import into your browser or OS trust store. See the main [README](../../README.md) → **First 5 Minutes**.

### 4. Node.js

Node.js 18+ is required for `npx` when using the published package.

### Environment variables

Set these in each host's MCP `env` block:

| Variable | Default | Purpose |
|----------|---------|---------|
| `SHADOWSCHEMA_EXPORT_URL` | `http://localhost:38081` | Export API base URL |
| `SHADOWSCHEMA_PROXY_URL` | `http://127.0.0.1:38080` | MITM proxy URL (document to agents for browser setup) |
| `SHADOWSCHEMA_AUTO_UPDATE` | `true` | When running from a git clone, fetch/pull and rebuild on startup |

If ShadowSchema runs on another machine (e.g. Docker on `10.10.10.89`), set both URLs to that host — export API on port **38081**, proxy on port **38080**.

---

## Launch command

### Run from source (current)

npm publish is **deferred** until manual testing is complete. Use a local clone:

Build once, then point your MCP config at the compiled entrypoint:

```bash
cd mcp && npm install && npm run build
```

```json
{
  "command": "node",
  "args": ["/path/to/shadowschema/mcp/dist/index.js"],
  "env": {
    "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
  }
}
```

> **Do not use `npm start` as the MCP command.** npm prints lifecycle banners to stdout, which breaks the stdio MCP handshake and leaves agents stuck on "initializing". Use `node dist/index.js` (or `npx tsx src/index.ts` with `cwd` set to `mcp/`) instead. `npm start` is fine for manual terminal testing only.

See [`examples/dev-from-repo.json`](../examples/dev-from-repo.json).

**Auto-update:** Enabled by default when running from a clone. On startup the MCP fetches the remote, pulls if behind, and runs `npm run build`. It skips quietly when the current branch has no upstream (e.g. local-only `dev`). Disable with `"SHADOWSCHEMA_AUTO_UPDATE": "false"` in the MCP `env` block if you do not want background git operations.

---

## Agent recipes

Ready-made prompts for common workflows: [recipes.md](recipes.md)

- **Map and build** — crawl, wait for endpoints, generate SDK, scaffold app
- **Auth + map** — login, vault, replay script
- **WebSocket API** — inbound/outbound schema discovery

---

## Grok Build

### Global config

Append to `~/.grok/config.toml`:

```toml
[mcp_servers.shadowschema]
command = "npx"
args = ["-y", "@notfixingit3/shadowschema-mcp"]
env = { SHADOWSCHEMA_EXPORT_URL = "http://localhost:38081" }
enabled = true
startup_timeout_sec = 60
```

Template: [`examples/grok-config.toml`](../examples/grok-config.toml)

### Project-scoped config (commit in repo)

Create `.grok/config.toml` in your project:

```toml
[mcp_servers.shadowschema]
command = "npx"
args = ["-y", "@notfixingit3/shadowschema-mcp"]
env = { SHADOWSCHEMA_EXPORT_URL = "http://localhost:38081" }
enabled = true
```

Template: [`examples/grok-project-config.toml`](../examples/grok-project-config.toml)

### CLI one-liner

```bash
grok mcp add shadowschema \
  -e SHADOWSCHEMA_EXPORT_URL=http://localhost:38081 \
  -- npx -y @notfixingit3/shadowschema-mcp
```

### Verify

```bash
grok mcp doctor shadowschema
```

In the Grok TUI, run `/mcps` and confirm `shadowschema` is enabled with tools listed.

### Tool naming

Grok namespaces MCP tools as `shadowschema__<tool_name>` — e.g. `shadowschema__list_endpoints`.

---

## OpenCode

Add to `opencode.json` or `opencode.jsonc` (project root or global config):

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "shadowschema": {
      "type": "local",
      "command": ["npx", "-y", "@notfixingit3/shadowschema-mcp"],
      "enabled": true,
      "timeout": 30000,
      "environment": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

Template: [`examples/opencode.jsonc`](../examples/opencode.jsonc)

### Usage

Reference the server by name in prompts:

> Use shadowschema to list endpoints for the active session, then describe the schema for `/api/v1/users`.

### Verify

```bash
opencode mcp list
```

### Tip

MCP tools add to context size. If you run many servers, disable `shadowschema` globally and enable it only on specific agents — see [OpenCode MCP docs](https://opencode.ai/docs/mcp-servers/).

---

## Cursor

### Project-scoped (recommended)

Create `.cursor/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "shadowschema": {
      "command": "npx",
      "args": ["-y", "@notfixingit3/shadowschema-mcp"],
      "env": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

Template: [`examples/cursor-mcp.json`](../examples/cursor-mcp.json)

### User-scoped

Cursor Settings → **MCP** → Add server with the same `command`, `args`, and `env`.

### Verify

Open the MCP panel in Cursor. `shadowschema` should show as connected. Ask the agent:

> Call shadowschema_list_sessions and show me the result.

---

## Claude Code

Merge into `~/.claude.json` under top-level `mcpServers`:

```json
{
  "mcpServers": {
    "shadowschema": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@notfixingit3/shadowschema-mcp"],
      "env": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

Template: [`examples/claude-code.json`](../examples/claude-code.json)

### Verify

Run `/mcp` in Claude Code to list connected servers.

---

## Claude Desktop

### Config file location

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |

Merge the `mcpServers` block:

```json
{
  "mcpServers": {
    "shadowschema": {
      "command": "npx",
      "args": ["-y", "@notfixingit3/shadowschema-mcp"],
      "env": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

Template: [`examples/claude-desktop.json`](../examples/claude-desktop.json)

### Verify

Restart Claude Desktop. Open **Settings → Connectors** (or **+** → Connectors). `shadowschema` should appear with its tools.

### Desktop extension (future)

A `.mcpb` desktop extension for one-click install is planned — see [`todo.md`](../../todo.md) Phase 4.

---

## VS Code (Copilot)

User-level config (macOS example):

`~/Library/Application Support/Code/User/mcp.json`

```json
{
  "servers": {
    "shadowschema": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@notfixingit3/shadowschema-mcp"],
      "env": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

Template: [`examples/vscode-mcp.json`](../examples/vscode-mcp.json)

> **Note:** VS Code / Copilot MCP config field names may vary by version (`servers` vs `mcpServers`). Confirm against your installed version.

---

## Antigravity

Antigravity 2.0, IDE, and CLI share a single global MCP config.

### Config file location

| OS | Path |
|----|------|
| macOS / Linux | `~/.gemini/config/mcp_config.json` |
| Windows | `%USERPROFILE%\.gemini\config\mcp_config.json` |

Merge the `shadowschema` entry into the top-level `mcpServers` block:

```json
{
  "mcpServers": {
    "shadowschema": {
      "command": "npx",
      "args": ["-y", "@notfixingit3/shadowschema-mcp"],
      "env": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

Template: [`examples/antigravity-mcp.json`](../examples/antigravity-mcp.json)

### Edit from the IDE

1. Open **MCP Servers** via the **...** dropdown at the top of the agent panel.
2. Click **Manage MCP Servers** → **View raw config**.
3. Add the `shadowschema` block, save, then click **Refresh** under **Installed MCP Servers**.

### Run from source (before npm publish)

Use a local clone instead of `npx`:

```json
{
  "mcpServers": {
    "shadowschema": {
      "command": "node",
      "args": ["/path/to/shadowschema/mcp/dist/index.js"],
      "env": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

See [`examples/dev-from-repo.json`](../examples/dev-from-repo.json). Do not use `npm start` here — see [Launch command](#launch-command).

### Verify

**CLI:** Run `/mcp` and confirm `shadowschema` is listed with its tools.

**IDE / Antigravity 2.0:** Open **Settings** → **Customizations** → **Installed MCP Servers** → **Refresh**. `shadowschema` should appear as connected.

Ask the agent:

> Call shadowschema_list_sessions and show me the result.

### Per-workspace config

Antigravity currently reads MCP servers from the global `mcp_config.json` only — it does not load project-level `.mcp.json` files. Use a distinct server name (e.g. `shadowschema-myproject`) if you need different settings per repo.

---

## Portable project config

Commit `.mcp.json` at your project root for hosts that read the MCP standard project format (Grok, some Claude tooling):

```json
{
  "mcpServers": {
    "shadowschema": {
      "command": "npx",
      "args": ["-y", "@notfixingit3/shadowschema-mcp"],
      "env": {
        "SHADOWSCHEMA_EXPORT_URL": "http://localhost:38081"
      }
    }
  }
}
```

Template: [`examples/mcp.json`](../examples/mcp.json)

For team repos, commit alongside `.grok/config.toml` and `.cursor/mcp.json` so everyone gets the server regardless of which agent they use. Antigravity users still need to merge the block into `~/.gemini/config/mcp_config.json` manually until per-workspace support lands.

---

## Companion MCPs

ShadowSchema MCP discovers APIs; you still need something to **generate traffic**. Pair with:

| Companion | Role |
|-----------|------|
| **Playwright / browser MCP** | Browse the target app through the MITM proxy |
| **Filesystem MCP** | Write generated SDKs and app scaffold to the workspace |
| **Fetch MCP** (optional) | Pull public docs to cross-check inferred schemas |

### Browser proxy settings

Route browser traffic through ShadowSchema:

| Setting | Value |
|---------|-------|
| HTTP proxy | `127.0.0.1:38080` |
| HTTPS proxy | `127.0.0.1:38080` |

Or set environment variables for CLI tools:

```bash
export HTTP_PROXY=http://127.0.0.1:38080
export HTTPS_PROXY=http://127.0.0.1:38080
```

---

## Autonomous exploration (`shadowschema_explore_target`)

Phase 3 adds a built-in Playwright crawl that routes traffic through the MITM proxy.

### One-time setup

```bash
cd mcp
npm install
npx playwright install chromium
```

### Basic crawl

```
1. shadowschema_create_session — target api.example.com
2. shadowschema_explore_target — start_url: https://app.example.com/, max_pages: 15
3. shadowschema_wait_for_endpoints — min_count: 3
4. shadowschema_list_endpoints
```

`explore_target` uses `ignore_https_errors: true` by default so the MITM CA does not need to be injected into Playwright separately.

### Authenticated crawl

Log in with a browser MCP first, save Playwright storage state to disk, then pass it to `explore_target`:

```
storage_state_path: /path/to/storage-state.json
```

Or log in manually in a headed browser through the proxy, export cookies, and reuse them via storage state.

---

## Example workflows

### Map and build

```
1. shadowschema_create_session — target api.example.com
2. Configure browser MCP to use proxy 127.0.0.1:38080
3. Browse the target app (or automate key flows)
4. shadowschema_list_endpoints
5. shadowschema_get_endpoint for each route you need
6. shadowschema_generate_sdk (typescript-fetch)
7. Scaffold an app using the generated client
```

### Auth + map

```
1. shadowschema_create_session — target api.example.com
2. Log in via browser MCP (through proxy)
3. shadowschema_get_vault — retrieve captured Authorization headers
4. shadowschema_list_endpoints — map authenticated routes
5. Build a client that injects vault credentials
```

### WebSocket API

```
1. Use the target app's real-time features through the proxy
2. shadowschema_list_endpoints — find ws:// / wss:// paths
3. shadowschema_get_endpoint — read x-websocket-message-schema-in/out
4. Build a WebSocket client matching inferred message shapes
```

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `ECONNREFUSED` on `:38081` | ShadowSchema not running | `docker compose up -d` |
| MCP server times out on start | `npx` downloading on first run | Increase startup timeout (Grok: `startup_timeout_sec = 60`) |
| Server connects but tools fail | MCP not built or wrong `cwd` | `cd mcp && npm install && npm run build`; use dev-from-repo config |
| Tools work but spec is empty | No traffic captured | Create a session, route traffic through `:38080`, browse the target |
| Endpoints from wrong host | Wrong active session | `shadowschema_switch_session` or create a new session |
| HTTPS sites fail in browser | CA not trusted | Import `shadowschema-ca.crt` into browser/OS |
| `localhost` fails on Linux Docker | Host networking quirk | Confirm port `38081` is published; try `127.0.0.1` explicitly |
| ShadowSchema on a remote host | MCP still points at `localhost` | Set `SHADOWSCHEMA_EXPORT_URL` and `SHADOWSCHEMA_PROXY_URL` to the remote IP (e.g. `http://10.10.10.89:38081`) |
| Auto-update logs on startup | Running from a local git clone | Expected when upstream exists; harmless. Set `SHADOWSCHEMA_AUTO_UPDATE=false` to disable |
| Stuck on "initializing" | MCP command is `npm start` | Use `node /path/to/shadowschema/mcp/dist/index.js` instead (`npm run build` first) |

### Quick health check

```bash
# Export API
curl -s http://localhost:38081/export-map | jq '.info.title'

# Sessions
curl -s http://localhost:38081/sessions | jq .

# Grok
grok mcp doctor shadowschema
```

---

## Security

ShadowSchema is a MITM tool. The MCP exposes what your local instance has captured.

- **Auth vault tools** return real tokens from intercepted traffic. Use only on systems you control.
- **Never commit** vault data, tokens, or `claude_desktop_config.json` with secrets to git.
- **Legal:** Only map APIs you own or have explicit permission to test. See the [Legal Disclaimer](../../README.md#legal-disclaimer) in the main README.
- **Local only:** The MCP talks to `localhost:38081` by default. Do not expose the export API to the public internet.

---

## Next steps

- Implement the MCP server — [`todo.md`](../../todo.md) Phase 1
- Publish `@notfixingit3/shadowschema-mcp` to npm
- Add a short link from the main [README](../../README.md) to this guide