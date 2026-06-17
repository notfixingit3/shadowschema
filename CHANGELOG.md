# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Changed
- **Hosted deployment:** Removed Traefik/preview section from public README and deleted `deploy/preview/`; stack configs and tester guide moved to private materials outside the repo.

## [1.1.2-beta.0] - 2026-06-17

Development branch opened for v1.1.2 on `dev`.

## [1.1.1] - 2026-06-17

Documentation-only stable release. No application code changes.

### Added
- **README screenshots:** Dashboard overview, endpoint detail, new session, Auth Vault, and Shadow Domains (`docs/screenshots/`).
- **Documentation:** Table of contents, architecture diagram, production checklist, troubleshooting guide, First 5 minutes walkthrough, OpenAPI export example, and Security & Data Handling section.
- **Doc tooling:** `scripts/seed-doc-demo.mjs`, `scripts/capture-doc-screenshots.mjs`, and `docker-compose.docs.yml` for local screenshot regeneration and alternate host ports.
- **CONTRIBUTING:** Release workflow and screenshot regeneration instructions.

### Changed
- **README:** Reorganized around Quick Start → Docker Deployment → Hosted → Development; expanded Docker image tag guide (`:beta`, `:latest`, `:vX.Y.Z`) with stable/beta pinning examples.
- **`.env.example` / `deploy/preview/.env.example`:** Commented blocks for beta, pinned stable, and rolling `:latest` tags.
- **Troubleshooting:** arm64 GHCR pull errors and Docker `127.0.0.1` vs `host.docker.internal` upstream note.

## [1.1.1-beta.3] - 2026-06-17

Documentation-only release. No application code changes.

### Added
- **README screenshots:** Dashboard overview, endpoint detail, new session, Auth Vault, and Shadow Domains (`docs/screenshots/`), captured from a local Docker stack with synthetic traffic.
- **First 5 minutes** walkthrough, trimmed OpenAPI export example, and Security & Data Handling section.
- **Doc tooling:** `scripts/seed-doc-demo.mjs`, `scripts/capture-doc-screenshots.mjs`, and `docker-compose.docs.yml` (alternate host ports + arm64 local-build notes).

### Changed
- **CONTRIBUTING:** Screenshot regeneration workflow.
- **Troubleshooting:** arm64 GHCR pull errors and Docker `127.0.0.1` vs `host.docker.internal` upstream note.

## [1.1.1-beta.2] - 2026-06-17

Documentation-only release. No application code changes.

### Added
- **README:** Table of contents, architecture (mermaid traffic-flow diagram), production checklist, and troubleshooting table.
- **CONTRIBUTING:** Release workflow (`dev` → `:beta` → `main` → `:vX.Y.Z` tag).

## [1.1.1-beta.1] - 2026-06-17

Documentation-only release. No application code changes.

### Changed
- **README:** Reorganized around Quick Start → Docker Deployment → Hosted → Development. Expanded image tag guide (`:beta`, `:latest`, `:vX.Y.Z`), stable/beta pinning examples, stack layout, env vars, update/rollback, and running-image checks.
- **`.env.example` / `deploy/preview/.env.example`:** Commented blocks for beta, pinned stable, and rolling `:latest` tags.

## [1.1.1-beta.0] - 2026-06-17

Development branch opened for v1.1.1 on `dev`.

## [1.1.0] - 2026-06-17

Stable release of the v1.1.0 beta cycle.

### Added
- **WebSocket / WSS recon:** Upgrade detection, frame capture, directional message schemas, and dashboard inspection.
- **Auth Vault:** Automatic capture and export of auth headers/tokens from intercepted traffic.
- **SDK generation:** One-click Python, TypeScript, Go, and Rust client zips from the dashboard.
- **Docker / GHCR deployment:** Pre-built proxy and dashboard images with compose stacks for local and Traefik hosting.
- **PostgreSQL persistence:** Production Docker stacks store sessions in Postgres; SQLite remains for local dev.
- **CA cert download:** Dashboard button and `GET /ca-cert` for MITM root trust setup.
- **Test & CI hardening:** Proxy integration tests, export API coverage, `go vet`, and `gosec` in CI.

### Changed
- Dashboard serves a production Vite build behind nginx with same-origin export API proxying.
- Pure-Go SQLite driver (no CGO); CI and release binaries build without gcc.

### Fixed
- PostgreSQL `pgx` driver registration, proxy latency via debounced spec persistence, Traefik/Vite host blocking, and SDK path-traversal hardening.

## [1.1.0-beta.9] - 2026-06-17

### Fixed
- **Proxy latency:** Debounce spec persistence (default 2s) so intercepted traffic no longer blocks on a full Postgres `UPDATE` per request/WebSocket frame.

## [1.1.0-beta.8] - 2026-06-16

### Added
- **CA cert download:** Dashboard **🔒 CA Cert** button and `GET /ca-cert` export API endpoint serve `shadowschema-ca.crt` for browser trust setup.

### Changed
- preview deployment docs: Postgres stack layout, first-time setup, migration note from SQLite volumes.

## [1.1.0-beta.7] - 2026-06-16

### Fixed
- PostgreSQL driver registration: use `pgx` stdlib driver name so Docker/CI smoke tests connect successfully.
- Gosec G706 log-injection finding on SQLite path logging.
- CI workflow still required CGO/gcc after the SQLite driver swap.

## [1.1.0-beta.6] - 2026-06-16

### Added
- **PostgreSQL persistence:** Docker stacks now run a `postgres:16-alpine` service; the proxy connects via `DATABASE_URL`.
- **Database abstraction:** `internal/spec/database.go` supports PostgreSQL (production) and SQLite (local dev/tests).

### Changed
- Replaced CGO-based `go-sqlite3` with pure-Go `modernc.org/sqlite` for local development.
- Docker images no longer require SQLite libraries or `/app/data` volumes; only `shadowschema-certs` (CA) and `shadowschema-postgres` (sessions) persist.
- CI builds and tests run without CGO.

## [1.1.0-beta.5] - 2026-06-16

### Changed
- Root `docker-compose.yml` and README now use pre-built GHCR images (`:beta`) instead of compiling with `go run` at container start.
- Local Docker stack serves the production dashboard at `http://localhost:8080` with same-origin export API via nginx.

## [1.1.0-beta.4] - 2026-06-16

### Added
- **Docker CI:** Publishes `ghcr.io/notfixingit3/shadowschema:beta` and `shadowschema-dashboard:beta` on every push to `dev`.
- **Production Dockerfiles:** Multi-stage proxy image (pre-built Go binary + Node for SDK generation) and static dashboard image (Vite build + nginx).

### Changed
- preview deployment now pulls GHCR `:beta` images instead of compiling with `go run` on the server.
- Dashboard preview serves a production Vite build instead of the Vite dev server.

## [1.1.0-beta.3] - 2026-06-16

### Added
- **Go and Rust SDK generation:** Dashboard and `/generate-sdk` now support OpenAPI Generator `go` and `rust` targets alongside Python and TypeScript.

## [1.1.0-beta.2] - 2026-06-16

### Added
- **preview deployment stack:** `deploy/preview/` with Traefik + nginx + docker-compose for hosting at `preview.example.internal`.
- **Vite dev proxy:** Dashboard dev server proxies export API routes to `:38081` for same-origin local development.

### Changed
- Dashboard uses same-origin API URLs (`VITE_API_URL`) so the export API works behind a reverse proxy.
- Welcome screen shows the live proxy hostname (e.g. `notfixingit:38080`) instead of hardcoded `localhost`.
- Root `docker-compose.yml` sets `GOTOOLCHAIN=auto` for Go 1.26+ module compatibility.

### Fixed
- Vite host blocking behind Traefik: nginx sends `Host: localhost` upstream; `allowedHosts` and HMR configured for HTTPS preview.

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