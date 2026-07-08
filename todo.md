# ShadowSchema MCP — Implementation Plan

> Bridge ShadowSchema's live API recon into coding agents (Grok Build, OpenCode, Cursor, Claude Code, etc.) via the Model Context Protocol.

## Goal

Let agents **discover undocumented APIs**, **read inferred schemas + auth context**, and **build apps/clients** without manual dashboard exports or curl scripts.

ShadowSchema already does the hard part (MITM → evolving OpenAPI). The MCP is a thin, agent-friendly adapter over the existing export API (`:38081`), plus optional orchestration for traffic generation.

---

## Architecture

```
┌─────────────────┐     MCP tools      ┌──────────────────┐
│  Coding Agent   │ ◄────────────────► │  shadowschema-mcp │
│ (Grok / OpenCode)│                    │  (new package)    │
└────────┬────────┘                    └────────┬─────────┘
         │                                      │ HTTP
         │ browser / playwright MCP             ▼
         ▼                             ┌──────────────────┐
┌─────────────────┐   MITM :38080      │  Export API       │
│  Target App     │ ──────────────────► │  :38081           │
└─────────────────┘                    │  (existing)       │
                                       └──────────────────┘
```

**Key principle:** ShadowSchema MCP = **discovery & spec bridge**. Traffic generation stays in browser/Playwright MCP (or a future optional crawl tool), routed through the MITM proxy.

---

## Phase 1 — Core MCP Server (MVP)

Thin MCP server wrapping existing export routes. Highest ROI, smallest scope.

### 1.1 Project scaffold

- [x] Add `mcp/` package (TypeScript + `@modelcontextprotocol/sdk`)
- [x] Config via env vars:
  - `SHADOWSCHEMA_EXPORT_URL` (default `http://localhost:38081`)
  - `SHADOWSCHEMA_PROXY_URL` (default `http://127.0.0.1:38080`) — documented for agent setup, not called directly
- [x] `package.json` with `start` / `build` scripts for MCP stdio transport
- [x] Agent setup guide — `mcp/docs/agent-setup.md` + `mcp/examples/`

### 1.2 MCP tools (map 1:1 to export API) ✅

| Tool | Backing endpoint | Purpose |
|------|------------------|---------|
| `shadowschema_health` | `GET /export-map` (lightweight ping) | Verify stack is up |
| `shadowschema_list_sessions` | `GET /sessions` | List recon sessions |
| `shadowschema_create_session` | `POST /sessions` | Start mapping a target host |
| `shadowschema_switch_session` | `POST /sessions/switch` | Activate a saved session |
| `shadowschema_get_spec` | `GET /export-map` | Full OpenAPI JSON/YAML |
| `shadowschema_list_endpoints` | derived from spec | Lightweight index: path, methods, extensions |
| `shadowschema_get_endpoint` | derived from spec | Single path: schema, `x-last-payload`, params, WS extensions |
| `shadowschema_get_vault` | `GET /vault` | Captured auth headers (with security warning in description) |
| `shadowschema_list_discovered_domains` | `GET /discovered` | Out-of-scope hosts seen via CONNECT |
| `shadowschema_add_target_domain` | `POST /sessions/add-target` | Expand interception perimeter |
| `shadowschema_generate_sdk` | `POST /generate-sdk` | Return SDK zip path or base64 blob |
| `shadowschema_get_ca_cert` | `GET /ca-cert` | Download MITM root CA for trust setup |

### 1.3 Agent-friendly output shaping

- [x] `list_endpoints` — return compact table, not full OpenAPI (token budget)
- [x] `get_endpoint` — include `x-last-payload`, `x-websocket-*` extensions when present
- [x] `get_spec` — support `format: json | yaml` and optional `path_prefix` filter (client-side filter OK for MVP)
- [x] Tool descriptions must state: **inferred/observed schemas, not authoritative docs**

### 1.4 MCP resources (optional but useful)

- [x] `shadowschema://spec/openapi.json` — live spec as a readable resource
- [x] `shadowschema://setup/proxy.md` — static guide: CA install, `HTTP_PROXY`/`HTTPS_PROXY`, Firefox/Chrome notes

### 1.5 Tests

- [x] Unit tests with mocked HTTP responses (no live ShadowSchema required)
- [x] Integration test script: `npm run test:integration` → exercise client against real export API
- [x] Document skip behavior when `:38081` is unreachable

### 1.6 Deliverable

Agent workflow (manual traffic for now):

1. User starts ShadowSchema (`docker compose up -d`)
2. Agent calls `shadowschema_create_session` → configures browser proxy via separate MCP
3. User/agent browses target app
4. Agent calls `shadowschema_list_endpoints` → `shadowschema_get_endpoint` → builds app

---

## Phase 2 — Polling & Agent Ergonomics

Reduce agent glue code (sleep loops, diffing, chunking).

### 2.1 New tools

- [x] `shadowschema_wait_for_endpoints` — poll until `min_count` or `path_prefix` match, with timeout
- [x] `shadowschema_spec_diff` — compare current spec hash/paths vs. last poll; return **new** endpoints only
- [x] `shadowschema_get_setup_status` — proxy reachable, export API reachable, active session name/target, endpoint count

### 2.2 Export API enhancements (Go backend)

Consider small additions to `internal/spec/spec.go` if client-side filtering is too heavy:

- [x] `GET /endpoints` — lightweight index endpoint (path, methods, last_seen, has_payload)
- [x] `GET /endpoints/{path...}` — single endpoint detail
- [x] `GET /export-map?path_prefix=/api/v1` — server-side filter
- [x] `GET /health` — explicit health check (export server up, session id, endpoint count)

### 2.3 Session scoping for agents

Current export API is mostly **active-session** scoped. For parallel agent work:

- [x] Add optional `session_id` query param to read endpoints/spec for non-active sessions (read-only)
- [x] Existing read tools (`get_spec`, `list_endpoints`, `get_endpoint`) accept optional `session_id`

---

## Phase 3 — Traffic Orchestration (Optional Crawl Layer)

Not a replacement for ShadowSchema — drives exploration **through** the MITM proxy.

### 3.1 `shadowschema_explore_target` tool (or separate `shadowschema-crawl` MCP)

- [x] Accept: `start_url`, `max_pages`, `max_depth`, `wait_ms`, `session_id`
- [x] Launch headless browser (Playwright) with `proxy: 127.0.0.1:38080`
- [x] Basic heuristics: follow same-origin links, click nav, wait for network idle
- [x] Return: pages visited, domains hit, endpoint count delta
- [x] **Requires** CA cert trusted in browser context (use `get_ca_cert` + Playwright `ignoreHTTPSErrors` or cert injection)

### 3.2 Auth-aware exploration

- [x] Document pattern: login via browser MCP first, then crawl authenticated routes
- [x] Optional: accept cookie jar / storage state file path for Playwright `storageState`

### 3.3 CLI replay integration

- [x] Expose dashboard's Python replay export as `shadowschema_export_replay_script` tool
- [x] `GET /export-replay` route in Go export API

---

## Phase 4 — Distribution & Docs

### 4.1 Packaging

- [x] ~~Publish as `npx @notfixingit3/shadowschema-mcp`~~ **DEFERRED** — hold until manual testing complete; run from source (`node dist/index.js`) for now
- [x] Optional: bundle MCP server in Docker Compose as a sidecar service (`docker compose --profile mcp`)
- [x] Add short MCP blurb + link to `mcp/docs/agent-setup.md` in main `README.md`

### 4.2 Agent setup documentation

Full guide: `mcp/docs/agent-setup.md` (committed, copy-paste configs per host).

- [x] **Prerequisites section** — ShadowSchema stack running (`docker compose up -d`), export API at `:38081`, Node.js 18+ for `npx`
- [x] **Quick verify** — `curl -s http://localhost:38081/export-map | head` and `grok mcp doctor shadowschema` (or host equivalent)
- [x] **Grok Build** — `~/.grok/config.toml` + project `.grok/config.toml` + `grok mcp add` CLI one-liner
- [x] **OpenCode** — `opencode.json` / `opencode.jsonc` local MCP block
- [x] **Cursor** — `.cursor/mcp.json` (project) and Cursor Settings → MCP (user)
- [x] **Claude Code** — `~/.claude.json` `mcpServers` block (stdio)
- [x] **Claude Desktop** — `claude_desktop_config.json` + optional `.mcpb` desktop extension (Phase 4 stretch)
- [x] **VS Code** — `mcp.json` under user settings (Copilot agent MCP)
- [x] **Portable project config** — repo-root `.mcp.json` for any host that reads the MCP standard format
- [x] **Committed examples** — `mcp/examples/` with sanitized config templates (no secrets)
- [x] **Troubleshooting** — connection refused, `npx` cold-start timeout, wrong `SHADOWSCHEMA_EXPORT_URL`, Docker host networking on Linux
- [x] **Security note** — vault tools expose captured auth; recommend local-only, never commit tokens
- [x] **README link** — add short MCP section to main `README.md` pointing to `mcp/docs/agent-setup.md`

### 4.3 Agent recipes (copy-paste prompts)

- [x] "Map and build" — [`mcp/docs/recipes.md`](mcp/docs/recipes.md)
- [x] "Auth + map" — [`mcp/docs/recipes.md`](mcp/docs/recipes.md)
- [x] "WebSocket API" — [`mcp/docs/recipes.md`](mcp/docs/recipes.md)

### 4.4 Security & legal callouts

- [x] Every vault/auth tool description warns: sensitive captured credentials, local-only, user consent required
- [x] Link to README legal disclaimer (`LEGAL_NOTE` in tool descriptions + agent-setup.md)
- [x] Never log token values in MCP server stdout (`redactVaultCredentials` helper for any future logging)

---

## Suggested file layout

```
mcp/
├── package.json
├── tsconfig.json
├── src/
│   ├── index.ts          # MCP server entry (stdio)
│   ├── client.ts         # HTTP client for :38081
│   ├── tools/
│   │   ├── sessions.ts
│   │   ├── spec.ts
│   │   ├── vault.ts
│   │   └── sdk.ts
│   ├── resources/
│   │   └── setup.ts
│   └── utils/
│       ├── filter.ts     # path_prefix, endpoint index
│       └── poll.ts       # wait_for_endpoints (Phase 2)
├── tests/
│   ├── client.test.ts
│   └── tools.test.ts
├── docs/
│   └── agent-setup.md    # Per-host MCP install guide
├── examples/
│   ├── grok-config.toml
│   ├── opencode.jsonc
│   ├── cursor-mcp.json
│   ├── claude-code.json
│   ├── claude-desktop.json
│   └── mcp.json          # Portable .mcp.json
└── README.md
```

---

## Agent Setup Documentation

> Deliverable: `mcp/docs/agent-setup.md` — one page, copy-paste configs. Examples also committed under `mcp/examples/`.

### Prerequisites (all hosts)

1. Start ShadowSchema: `docker compose up -d`
2. Confirm export API: `curl -s http://localhost:38081/export-map`
3. (Optional) Trust MITM CA for browser traffic — see main `README.md` → First 5 Minutes

**Shared env vars** (set in each host's MCP `env` block):

| Variable | Default | Purpose |
|----------|---------|---------|
| `SHADOWSCHEMA_EXPORT_URL` | `http://localhost:38081` | Export API base URL |
| `SHADOWSCHEMA_PROXY_URL` | `http://127.0.0.1:38080` | Documented for agent proxy setup; not used by MCP server directly |

**Recommended launch command** (until published to npm):

```json
["npx", "-y", "tsx", "/path/to/shadowschema/mcp/src/index.ts"]
```

After publish:

```json
["npx", "-y", "@notfixingit3/shadowschema-mcp"]
```

---

### Grok Build

**Global** — `~/.grok/config.toml`:

```toml
[mcp_servers.shadowschema]
command = "npx"
args = ["-y", "@notfixingit3/shadowschema-mcp"]
env = { SHADOWSCHEMA_EXPORT_URL = "http://localhost:38081" }
enabled = true
startup_timeout_sec = 60   # npx cold-start on first run
```

**Project-scoped** (commit in repo) — `.grok/config.toml`:

```toml
[mcp_servers.shadowschema]
command = "npx"
args = ["-y", "@notfixingit3/shadowschema-mcp"]
env = { SHADOWSCHEMA_EXPORT_URL = "http://localhost:38081" }
enabled = true
```

**CLI one-liner:**

```bash
grok mcp add shadowschema \
  -e SHADOWSCHEMA_EXPORT_URL=http://localhost:38081 \
  -- npx -y @notfixingit3/shadowschema-mcp
```

**Verify:** `grok mcp doctor shadowschema` — then `/mcps` in TUI to confirm tools load.

**Tool naming:** Grok namespaces tools as `shadowschema__<tool_name>` (e.g. `shadowschema__list_endpoints`).

---

### OpenCode

**Config file:** `opencode.json` or `opencode.jsonc` (project root or global).

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

**Usage in prompts:** reference by server name — e.g. *"use shadowschema to list endpoints for the active session"*.

**Verify:** `opencode mcp list`

**Tip:** If tool count is high, disable globally and enable per-agent — see [OpenCode MCP docs](https://opencode.ai/docs/mcp-servers/).

---

### Cursor

**Project-scoped** (recommended, commit in repo) — `.cursor/mcp.json`:

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

**User-scoped:** Cursor Settings → MCP → Add server (same `command` / `args` / `env`).

**Verify:** MCP panel shows `shadowschema` connected; ask agent to call `shadowschema_list_sessions`.

---

### Claude Code

**Config file:** `~/.claude.json` → top-level `mcpServers`:

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

**Project override:** some versions support per-project `mcpServers` in `.claude/settings.local.json` — document if supported at ship time.

**Verify:** `/mcp` in Claude Code to list connected servers.

---

### Claude Desktop

**Config file:**

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |

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

**Verify:** Settings → Connectors (or **+** → Connectors) shows `shadowschema` with tools.

**Stretch goal:** package as `.mcpb` desktop extension for one-click install from Claude Desktop Settings → Extensions.

---

### VS Code (GitHub Copilot agent MCP)

**Config file:** `~/Library/Application Support/Code/User/mcp.json` (macOS) or equivalent.

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

> Confirm exact schema against installed VS Code / Copilot version at ship time — field names may differ (`mcpServers` vs `servers`).

---

### Portable project config (`.mcp.json`)

For hosts that read the MCP standard project config (Grok, some Claude tooling):

**Repo root** — `.mcp.json`:

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

Commit this alongside `.grok/config.toml` and `.cursor/mcp.json` so teams pick up the server regardless of agent.

---

### Recommended companion MCPs

Document pairing ShadowSchema with browser/traffic MCPs for the full "map → build" loop:

| Companion | Role |
|-----------|------|
| Playwright / browser MCP | Generate traffic through MITM proxy (`HTTP_PROXY=http://127.0.0.1:38080`) |
| Filesystem MCP | Write generated SDKs and app scaffold to workspace |
| (optional) Fetch MCP | Pull public docs to cross-check inferred schemas |

Include a **combined workflow** snippet in `agent-setup.md`:

1. `shadowschema_create_session` → target `api.example.com`
2. Configure browser MCP proxy → `127.0.0.1:38080`
3. Browse / automate login
4. `shadowschema_wait_for_endpoints` (Phase 2) or `shadowschema_list_endpoints`
5. `shadowschema_generate_sdk` → build app

---

### Troubleshooting (document in agent-setup.md)

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `ECONNREFUSED :38081` | ShadowSchema not running | `docker compose up -d` |
| MCP server times out on start | `npx` downloading on first run | Increase startup timeout (Grok: `startup_timeout_sec = 60`) |
| Tools load but return empty spec | No traffic captured yet | Browse target through proxy; create session first |
| Endpoints on wrong host | Active session mismatch | `shadowschema_switch_session` or create new session |
| Linux Docker networking | `localhost` not reachable from host MCP | Use `host.docker.internal` or published port mapping |

---

## Success criteria

| Milestone | Done when |
|-----------|-----------|
| **MVP** | Agent lists endpoints and retrieves schema + sample payload from a live session without curl ✅ |
| **Phase 2** | Agent waits for new endpoints after browse without manual polling loops ✅ |
| **Phase 3** | Agent triggers basic crawl and sees endpoint count grow autonomously ✅ |
| **Ship** | One-line MCP install documented; works with Docker Compose stack out of the box ✅ |
| **Docs** | `mcp/docs/agent-setup.md` covers Grok Build, OpenCode, Cursor, Claude Code, Claude Desktop, VS Code, Antigravity with copy-paste configs ✅ |

---

## Out of scope (for now)

- Replacing Playwright/browser MCP
- Active API fuzzing or authenticated brute-force endpoint discovery
- Cloud-hosted ShadowSchema (MCP assumes local or self-hosted export API)
- Official OpenAPI validation / linting (agents can chain existing tools)

---

## Decisions & Closed Questions

- [x] **TypeScript vs Python for MCP server?** TypeScript. It aligns with the dashboard toolchain and makes sharing node modules / schemas cleaner.
- [x] **Should `generate_sdk` write zip to workspace temp dir or return as MCP embedded resource?** Implemented both. The tool accepts `output: "base64"` or `output: "path"` with `write_path` argument.
- [x] **Add `session_id` to all export routes now, or defer until multi-agent demand is clear?** Added `session_id` to health, spec, and endpoints routes on the Go backend and supported it in the MCP client/server.
- [x] **Separate repo vs `mcp/` subfolder in this monorepo?** Retained as `mcp/` subfolder in this monorepo to simplify development, testing, and deployment.

---

## References

- Export API (existing): `README.md` → Spec Extraction
- Route handlers: `internal/spec/spec.go` → `mountExportRoutes`
- SDK generation: `internal/spec/export.go`, `internal/spec/sdk.go`
- MCP SDK: https://github.com/modelcontextprotocol/typescript-sdk

---

# Post–v1.1.3-beta.9 Roadmap

> Captured after shipping recon hardening (request bodies, secret-safe export, host matching, status codes, path params/servers, export auth/CORS, richer templates/schema merge, vault coverage) plus HAR import, GraphQL ops index, and Explore 2.0.
>
> Priority bands: **P1** = high value / correctness, **P2** = product leverage, **P3** = polish / scale.

## Shipped (context — do not re-open without cause)

- [x] Request body capture + replay uses request body only
- [x] Default export omits vault secrets (`?include_secrets=1` / `/vault`)
- [x] Exact/subdomain `IsTarget` (no substring)
- [x] Path parameters + OpenAPI `servers`
- [x] Real status codes + empty 2xx + error bodies
- [x] Session create/switch/delete error handling
- [x] Export bind docs + token auth + CORS allowlist + localhost compose ports
- [x] Richer path templating + schema merge (nullable, oneOf, formats)
- [x] Cookie / Set-Cookie / CSRF vault headers
- [x] HAR import (`POST /import-har`, dashboard, MCP)
- [x] GraphQL `x-graphql-operations` index
- [x] Explore 2.0 (networkidle, seeds, clicks, allow_hosts)

---

## P1 — Correctness & security

### Schema & capture quality

- [x] **Multi-sample response history** — `x-payload-samples` (last 5), `x-hit-count`, `x-status-histogram`
- [x] **Required query/header inference** — `required: true` after ≥3 consistent hits with zero misses
- [x] **Query/header typing** — integer/bool/uuid formats for query params from observed values
- [ ] **Content-Type aware bodies** — form-urlencoded, multipart field names, protobuf/binary hints (don’t force JSON string schema)
- [ ] **Non-JSON media types** — record `text/*`, XML, protobuf as distinct content types in OpenAPI
- [ ] **Path param collisions** — when two segments template to the same name (`/{id}/…/{id}`), generate unique names (`id`, `id2`) or semantic names from neighbors
- [ ] **Ignore-rule UX** — validate ignore regex on session create; preset packs (static assets, analytics, well-known noise)

### Auth & secrets

- [x] **Vault redaction modes (MCP)** — `get_vault({ include_values: false })` default; REST `/vault` redacts unless `include_values=1`
- [x] **Scoped vault per host** — credentials keyed by host; `/vault?host=` + MCP filter
- [ ] **Cookie jar model** — parse Set-Cookie into named cookies; rebuild Cookie header for replay without dumping full jar into every export
- [x] **Strip secrets from SDK zip** — `specForSDK` / `sanitizeDocForExport` strips vault + sample payloads
- [ ] **Dashboard XSS hardening** — use `textContent` / `escapeHtml` for all path/param/summary fields injected into HTML

### Export API & ops

- [ ] **Multi-arch Docker images** — publish `linux/amd64` + `linux/arm64` (Apple Silicon friction)
- [ ] **Compose profile `public-ports`** — keep localhost bind as default; opt-in `0.0.0.0` for lab devices
- [ ] **Export API rate limits / body caps** — protect `/import-har` and `/generate-sdk` on shared hosts
- [ ] **Health that doesn’t require token** — optional unauthenticated `/health` when `SHADOWSCHEMA_EXPORT_TOKEN` is set (for orchestrators)
- [ ] **Postgres migration path** — document/tooling for SQLite → Postgres for users who started on local dev

---

## P2 — Product leverage (agent + recon)

### Agent / MCP

- [ ] **Publish npm package** — `@notfixingit3/shadowschema-mcp` (was deferred until manual testing complete)
- [x] **`shadowschema_validate_spec`** — kin-openapi + path-param / servers checks (`GET /validate-spec`)
- [x] **`shadowschema_session_diff`** — diff two session specs (`GET /sessions/diff`)
- [ ] **Webhook / SSE “new endpoints” stream** — push instead of poll-only `wait_for_endpoints` / `spec_diff`
- [ ] **MCP vault safety** — never log token values; optional auto-redact tool results when host agent has logging enabled
- [ ] **Auto-update default off for dirty trees** — or limit auto-update to `mcp/` package only (avoid whole monorepo `git pull`)
- [ ] **Explore: auth form helper** — optional login URL + selector map, or document storageState recipes more deeply
- [ ] **Explore: network log harvest** — record XHR/fetch URLs seen during crawl even without full body (candidate endpoints)

### GraphQL & special protocols

- [ ] **GraphQL → pseudo-paths** — optional map `query:GetUser` → `/graphql#GetUser` or OpenAPI path extensions for cleaner agent lists
- [ ] **GraphQL schema introspection** — if `__schema` allowed, import types (opt-in, noisy)
- [ ] **gRPC / Connect-RPC detection** — mark binary content + path; don’t pretend JSON schema
- [ ] **Server-Sent Events (SSE)** — detect `text/event-stream`, capture event names if JSON

### Dashboard

- [ ] **Modular dashboard** — split `main.js` into modules; add smoke tests for render helpers
- [ ] **Request body tab** — first-class UI (already partially in raw panel)
- [ ] **GraphQL ops panel** — expandable per operation with variables + last response
- [ ] **Endpoint notes/tags** — operator annotations (`auth`, `admin`, `payments`) stored in session extensions for agents
- [ ] **Hit counts & last status chips** — volume / reliability signal in sidebar
- [ ] **HAR drag-and-drop** — full drop zone + progress for large files
- [ ] **PWA offline spec cache** — last export available without export API

### Replay & codegen

- [ ] **Replay multi-language** — curl, HTTPie, TypeScript `fetch`, Go, in addition to Python
- [ ] **Path param substitution in replay** — fill `{id}` from last observed raw path or example values
- [ ] **OpenAPI examples** — populate `example` / `examples` from last payloads (redacted)
- [ ] **SDK post-process** — inject base URL from `servers` and document vault header wiring

---

## P3 — Hardening, distribution, polish

### Recon depth

- [ ] **Certificate pinning / mTLS playbook** — docs + detect TLS failures that look like pinning
- [ ] **HTTP/2 push / trailers** — capture if go-mitmproxy exposes them
- [ ] **Redirect chain mapping** — optional 3xx as linked operations (currently skipped)
- [ ] **Frequency-based path templating** — learn “this segment varies a lot under this prefix → template”
- [ ] **OpenAPI 3.1 option** — nullable/type arrays native; dual export mode

### Testing & quality

- [ ] **Integration suite for HAR fixtures** — real-world truncated HARs in `testdata/`
- [ ] **GraphQL fixture matrix** — batch ops, anonymous shorthand, mutations, subscriptions
- [ ] **Proxy golden tests** — end-to-end MITM with mock backend for request body + status codes
- [ ] **Dashboard e2e** — Playwright against docker stack (import HAR, open endpoint, export)

### Release & docs

- [ ] **arm64 GHCR builds in CI**
- [ ] **Version surface area** — single source of truth for app/MCP versions (today CHANGELOG + mcp/package.json)
- [ ] **Threat model doc** — MITM CA, vault, export API, agent access (one page for security reviews)
- [ ] **“First capture” tutorial video/GIF** — HAR path vs live MITM path
- [ ] **Stable v1.1.3 cut** — when beta.9+ soaks: merge to main, tag `:v1.1.3`, pin README examples

### Explicitly still out of scope (revisit later)

- [ ] Active API fuzzing / brute-force discovery
- [ ] Cloud multi-tenant hosted ShadowSchema
- [ ] Replacing Burp/ZAP as full proxy workbench
- [ ] Automated exploit generation

---

## Suggested next sprint (opinionated order)

1. [x] **Vault redaction modes + strip secrets from SDK input** — `/vault?include_values=`, MCP default redact, `specForSDK` sanitizes samples/vault
2. [x] **Multi-sample / required-param inference** — `x-hit-count`, `x-payload-samples`, `x-param-stats`, typed query params, required after consistent hits
3. [x] **Session diff + OpenAPI validate MCP tools** — `GET /sessions/diff`, `GET /validate-spec`, MCP `session_diff` + `validate_spec`
4. [x] **Multi-arch Docker** — CI publishes amd64 + arm64
5. **npm publish MCP** — once soak tests pass on beta.9
6. [x] **Dashboard modularization + GraphQL ops panel** — modules under `dashboard/modules/`, GraphQL tab
7. **Stable v1.1.3** after soak
- [x] **Scoped vault per host** — vault keyed by host; filter via `?host=`

---

## Notes for implementers

- Prefer extending export API first, then thin MCP wrappers (same pattern as Phase 1–3).
- Keep “inferred not authoritative” language on any new schema tools.
- Never put live tokens in default export artifacts or commit-friendly outputs.
- HAR + live MITM should remain feature-parity for capture fields (request body, status, GraphQL, vault).