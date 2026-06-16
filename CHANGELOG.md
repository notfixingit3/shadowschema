# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [1.1.0-beta.1] - 2026-06-16

### Added
- **Proxy integration tests:** `main_test.go` covers port availability, JSON interception, vault capture, WebSocket upgrade registration, and discovered-domain tracking through the live MITM proxy.
- **Export API test coverage:** HTTP tests for session list/create/switch/delete, add-target, discovered domains, YAML export, CORS preflight, and SDK zip generation.
- **Spec manager unit tests:** Ignore rules, HTTP methods, non-JSON payloads, query/header capture, target matching, and `ExportJSON` file output.
- **Isolated test databases:** Spec and main tests run in per-test temp directories to avoid polluting `./shadowschema.db`.

### Changed
- Extracted `newProxyServer()` from `main()` so the proxy pipeline is testable without starting the full process.
- SDK generation moved to `internal/spec/sdk.go` with allowlisted languages and root-scoped zip reads via `os.OpenRoot`.

### Security
- CI now enforces `go vet` and `gosec` on every push and PR.
- SDK zip generation hardened against path traversal; generated files use `0600` permissions.

## [1.1.0-beta.0] - 2026-06-16

### Added
- **WebSocket / WSS Support:** Detects upgrade handshakes on target domains, maps them in the dashboard as `WS` endpoints, and tags them with `x-websocket: true` in exported OpenAPI specs.
- **WebSocket Frame Inspection:** Taps post-101 connections to capture text/binary frames, stores recent frame history (`x-websocket-frames`), and infers evolving JSON message schemas. Supports RFC 6455 fragment reassembly, control-frame capture (ping/pong/close), binary payloads, and per-endpoint live stats (`x-websocket-stats`).
- **Directional WebSocket Schemas:** Separate inbound (`x-websocket-message-schema-in`) and outbound (`x-websocket-message-schema-out`) payload shape inference.
- **Legacy Migration:** Automatically migrates older `trace`-based WebSocket entries in SQLite sessions to `get` + `x-websocket` on load.
- **SDK Safety:** OpenAPI SDK generation excludes WebSocket endpoints and reports omissions via the `X-ShadowSchema-WebSocket-Excluded` response header.
- **Vault-Aware Replay:** `/export-map` embeds captured Auth Vault credentials (`x-shadowschema-vault`) and OpenAPI `securitySchemes`; Python replay scripts auto-inject vault headers.
- **Auth Vault:** Automatic capture and dashboard review of `Authorization`, API keys, and session tokens from intercepted traffic.
- **YAML Export & SDK Generation:** One-click OpenAPI YAML export and Python/TypeScript SDK zip downloads from the dashboard.

### Changed
- WebSocket endpoints use `GET` + `x-websocket` instead of overloading OpenAPI `TRACE`.
- Dashboard WebSocket detail view shows live frame stats, a frame log, and split inbound/outbound schemas.

### Fixed
- Dashboard detail panel now scrolls back to the top when selecting a different endpoint from the sidebar.

## [1.0.0] - 2026-06-16

### Added
- **Core Engine:** Automated HTTP/HTTPS proxy using `elazarl/goproxy`.
- **Security:** Dynamic CA generation with `crypto/x509` and automatic trust bridging.
- **Routing:** Basic API path deduplication using regular expressions (`/{uuid}`, `/{id}`, `/{year}`).
- **Schema Mapping:** JSON schema inference engine capable of automated type detection and recursive schema evolution.
- **Data Persistence:** SQLite integration to persist OpenAPI specifications across restarts (`shadowschema.db`).
- **Telemetry:** Proxy intercepts and maps URL query parameters and custom HTTP headers automatically.
- **UI:** Real-time glassmorphism-styled Vite + Vanilla JS web dashboard.
- **Exporting:** OpenAPI specification management with background export server on `:38081` and CORS support.
- **Shadow Domains:** Out-of-scope domain discovery with one-click target expansion.
- **Noise Cancellation:** Regex-based ignore rules for static assets and telemetry paths.
- **Python Replay:** One-click copy of intercepted endpoints as Python `requests` scripts.

### Changed
- Moved default proxy port to `:38080` and export port to `:38081` to prevent standard port collisions.