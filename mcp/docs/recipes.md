# ShadowSchema MCP — Agent Recipes

Copy-paste prompts for coding agents. Requires ShadowSchema running (`docker compose up -d`) and the MCP configured — see [agent-setup.md](agent-setup.md).

> **Legal:** Only use these workflows on APIs and applications you own or have explicit written permission to test. See the [Legal Disclaimer](../../README.md#legal-disclaimer).

---

## Map and build

Discover an undocumented API and scaffold a typed client app.

```
I want to map and build against an undocumented API using ShadowSchema.

1. shadowschema_get_setup_status — confirm export API and proxy are reachable
2. shadowschema_create_session — name: "My App Map", target: api.example.com
3. shadowschema_explore_target — start_url: https://app.example.com/, max_pages: 20, max_depth: 2
   (or browse manually through proxy 127.0.0.1:38080 if explore is unavailable)
4. shadowschema_wait_for_endpoints — min_count: 5, path_prefix: /api, timeout_ms: 120000
5. shadowschema_list_endpoints — show me the discovered routes
6. For the 3 most interesting endpoints, shadowschema_get_endpoint and summarize request/response shapes
7. shadowschema_generate_sdk — language: typescript-fetch, output: path, write_path: ./generated-sdk.zip
8. Scaffold a minimal app in this workspace that uses the generated client against the mapped endpoints

Treat all schemas as inferred from observed traffic, not official documentation.
```

---

## Auth + map

Map authenticated routes after login, then produce a replay script.

```
Map authenticated API routes for api.example.com using ShadowSchema.

1. shadowschema_create_session — name: "Auth Map", target: api.example.com
2. Tell me to log in via browser through proxy 127.0.0.1:38080 (or use storage_state_path if I have Playwright state)
3. After login, shadowschema_explore_target — start_url: https://app.example.com/dashboard, max_pages: 15
4. shadowschema_get_vault — list captured auth header names only; do not paste token values into files or git
5. shadowschema_list_endpoints — filter mentally to authenticated /api routes
6. shadowschema_export_replay_script — pick the most important POST endpoint (path + method)
7. Save the replay script to scripts/replay_example.py (redact tokens in comments if needed)

Only proceed if I confirm I have permission to test this API.
```

---

## WebSocket API

Discover and document WebSocket message shapes.

```
Map WebSocket APIs for api.example.com with ShadowSchema.

1. shadowschema_create_session — target: api.example.com
2. I'll use the app's real-time features through proxy 127.0.0.1:38080 (chat, live feed, etc.)
3. shadowschema_wait_for_endpoints — min_count: 1, timeout_ms: 60000
4. shadowschema_list_endpoints — find entries with websocket: true
5. For each WebSocket path, shadowschema_get_endpoint and extract:
   - x-websocket-message-schema-in
   - x-websocket-message-schema-out
   - x-websocket-frames if present
6. Propose a TypeScript WebSocket client interface matching the inferred schemas

Note: WebSocket schemas evolve as more frames are observed — treat them as best-effort.
```

---

## Quick health check

```
Use shadowschema_get_setup_status and shadowschema_health.
Tell me if ShadowSchema is ready for mapping, and what session is active.
```

---

## Diff-only polling

Useful while manually browsing — agent watches for new routes without full re-list.

```
I'm browsing a target through the ShadowSchema proxy.
Every 30 seconds, call shadowschema_spec_diff and tell me only new_paths.
Stop when I say stop.
```