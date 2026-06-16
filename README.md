<div align="center">
  <img src="logo.jpg" alt="ShadowSchema Logo" width="300" />
  <br/>
  <p><b>Advanced API MITM Mapping & Reconnaissance Framework</b></p>
</div>

<p align="center">
  <a href="https://github.com/notfixingit3/shadowschema/actions"><img src="https://github.com/notfixingit3/shadowschema/actions/workflows/build.yml/badge.svg" alt="Build Status"></a>
  <a href="https://github.com/notfixingit3/shadowschema/actions"><img src="https://github.com/notfixingit3/shadowschema/actions/workflows/docker.yml/badge.svg" alt="Docker CI"></a>
  <a href="https://goreportcard.com/report/github.com/notfixingit3/shadowschema"><img src="https://goreportcard.com/badge/github.com/notfixingit3/shadowschema" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/sponsors/notfixingit3"><img src="https://img.shields.io/badge/sponsor-30363D?style=flat&logo=GitHub-Sponsors&logoColor=#ea4aaa" alt="Sponsor"></a>
</p>

---

## 👁️ Overview

**ShadowSchema** is a specialized, clandestine Man-in-the-Middle (MITM) proxy engineered in Go. Designed for advanced API reconnaissance, it silently intercepts target HTTP/HTTPS telemetry, deduces underlying JSON payloads, and programmatically reconstructs evolving OpenAPI 3.0 specifications on the fly.

Built for red teamers, security researchers, and systems architects who need to map undocumented endpoints in real-time.

## ⚡ Core Capabilities

- **Deep TLS Inspection:** Deploys a dynamically generated local Certificate Authority (CA) on startup, effortlessly bypassing HTTPS encryption to inspect application layers.
- **Heuristic Schema Inference:** Parses intercepted JSON telemetry recursively, performing automated type detection and bridging schema mutations iteratively.
- **Intelligent Routing Deduplication:** Aggregates variable routes through regex-driven pattern matching (UUIDs, IDs, Timestamps), drastically reducing map noise.
- **Shadow Domains Tracking:** Automatically detects when the target client communicates with out-of-scope APIs (like CDNs or third-party telemetry) and allows you to instantly add them to your interception perimeter.
- **Noise Cancellation:** Supports regex-based ignore rules to filter out static assets (`.png`, `.css`) or telemetry paths.
- **WebSocket & WSS Recon:** Detects `ws://` and `wss://` upgrade handshakes, deduplicates socket paths, captures `Sec-WebSocket-*` headers and query params, reassembles fragmented frames, logs ping/pong/close control traffic, and infers evolving JSON message schemas from live text/binary payloads.
- **Raw Payload Capture:** In addition to inferring the structural schema, ShadowSchema captures the last seen raw JSON payload for each endpoint so you can inspect actual live data alongside inferred types.
- **Dynamic Python Replay:** Includes a one-click exporter that parses an intercepted endpoint and its last seen payload directly into a functioning Python `requests` script to immediately replicate API calls.
- **SDK Generation:** One-click OpenAPI client SDK zips for Python, TypeScript, Go, and Rust via OpenAPI Generator.
- **Persistent Sessions:** Automatically stores mapped endpoints and active sessions in PostgreSQL (Docker) or SQLite (local dev), ensuring recon sessions survive shutdowns and restarts.
- **Progressive Web App (PWA):** Features a sleek, beautiful dashboard to manage target sessions, filter endpoints, and export specifications as JSON.

## 🛠️ Infrastructure Requirements

- **Docker users:** Docker Engine with Compose v2 — includes PostgreSQL; no Go or Node.js required.
- **Contributors:** Go 1.21+ and Node.js 20+ for local development (see `CONTRIBUTING.md`). Local `go run` uses SQLite; Docker uses PostgreSQL.
- **Privileges:** Root CA (`certs/ca.crt`) installation capabilities to satisfy client-side SSL validation constraints.

## 🐳 Docker Images

Pre-built images are published to [GitHub Container Registry](https://github.com/notfixingit3/shadowschema/pkgs/container/shadowschema):

| Image | Description |
|-------|-------------|
| `ghcr.io/notfixingit3/shadowschema` | MITM proxy + export API (`:38080`, `:38081`) |
| `ghcr.io/notfixingit3/shadowschema-dashboard` | Production dashboard (static Vite build + nginx on `:8080`) |

| Tag | When published |
|-----|----------------|
| `:beta`, `:dev` | Every push to `dev` |
| `:latest`, `:main` | Every push to `main` |
| `:v1.1.0-beta.7` | Git tags (semver) |

Pin a release with `SHADOWSCHEMA_IMAGE=ghcr.io/notfixingit3/shadowschema:v1.1.0-beta.7` (see `.env.example`).

## 🚀 Deployment & Installation

### Option 1: Docker Compose (Recommended)

Pull pre-built images and run the full stack locally — no compilation on your machine.

```bash
git clone https://github.com/notfixingit3/shadowschema.git
cd shadowschema
docker compose pull
docker compose up -d
```

| Service | URL |
|---------|-----|
| Dashboard | http://localhost:8080 |
| MITM proxy | `127.0.0.1:38080` |
| Export API | http://localhost:38081 |

Session data persists in the `shadowschema-postgres` volume. The MITM CA keypair persists in `shadowschema-certs`.

Copy `.env.example` to `.env` to customize Postgres credentials or pin image tags.

To update after a new release:

```bash
docker compose pull && docker compose up -d
```

To copy the CA cert out of a running container:

```bash
docker cp shadowschema-proxy:/app/certs/ca.crt ./ca.crt
```

### Option 2: preview Preview (Traefik)

For hosting behind Traefik on the `preview.me` network, use `deploy/preview/`. The stack runs:

| Service | Role |
|---------|------|
| `postgres` | Session + spec persistence |
| `proxy` | MITM on `:38080`, export API on `:38081` |
| `dashboard` | Static UI (nginx, internal `:8080`) |
| `nginx` | Same-origin front for Traefik (`preview.example.internal`) |

**First-time setup:**

```bash
cd deploy/preview
cp .env.example .env
# Set POSTGRES_PASSWORD in .env before going live
docker compose pull
docker compose up -d
```

**Updates** (after a `dev` push or new tag):

```bash
cd /opt/stacks/shadowschema_preview   # or your checkout's deploy/preview
docker compose pull && docker compose up -d
```

Pin a known-good release in `.env`:

```
SHADOWSCHEMA_IMAGE=ghcr.io/notfixingit3/shadowschema:v1.1.0-beta.7
SHADOWSCHEMA_DASHBOARD_IMAGE=ghcr.io/notfixingit3/shadowschema-dashboard:v1.1.0-beta.7
```

Live preview: https://preview.example.internal

> **Migrating from pre-beta.6:** Older stacks used a `shadowschema-data` SQLite volume. Beta.6+ requires Postgres — the first deploy creates a fresh database. CA certs in `shadowschema-certs` are preserved.

### Option 3: Build from Source (Contributors)

For hacking on the proxy or dashboard, run the dev toolchain directly. By default, the proxy loads your last active session from the SQLite database.

```bash
# MITM engine
go run main.go
```

```bash
# Dashboard dev server (proxies export API to :38081)
cd dashboard
npm install
npm run dev
```

Navigate to `http://localhost:5173`. From the dashboard you can create Target Sessions, manage noise cancellation rules, explore Shadow Domains, and inspect WebSocket endpoints.

To build images locally:

```bash
docker build -t shadowschema:local .
docker build -f Dockerfile.dashboard -t shadowschema-dashboard:local .
```

## 🎮 Usage Examples

### 1. Intercepting a Mobile App (iOS/Android)
1. Start ShadowSchema and navigate to the dashboard to create a new session targeting `api.targetapp.com`.
2. Transfer `certs/ca.crt` to your mobile device and install it as a trusted Root CA.
3. Configure your mobile device's Wi-Fi settings to use your computer's local IP (e.g., `192.168.1.10:38080`) as an HTTP Proxy.
4. Open the app! The dashboard will instantly populate with mapped endpoints as you navigate through it.

### 2. Intercepting cURL Requests
You can route CLI tools through the proxy using environment variables. The `-k` flag is required if you haven't installed the CA cert to your system's trust store.
```bash
export http_proxy=http://127.0.0.1:38080
export https_proxy=http://127.0.0.1:38080

# Assuming your active session targets "example.com"
curl -k https://example.com/api/v1/users
```

### 3. Intercepting a Browser (Firefox Developer Edition)
Firefox Developer Edition is excellent for API recon because it has its own independent certificate store and proxy settings, keeping your daily browsing unaffected.

**Configure the Proxy:**
1. Open Firefox Developer Edition and go to **Settings** -> **General** -> **Network Settings** (at the very bottom).
2. Select **Manual proxy configuration**.
3. Set **HTTP Proxy** to `127.0.0.1` and **Port** to `38080`.
4. Check **Also use this proxy for HTTPS**.
5. Click **OK**.

**Trust the ShadowSchema CA:**
1. In Settings, go to **Privacy & Security**.
2. Scroll down to the **Certificates** section and click **View Certificates...**.
3. Go to the **Authorities** tab and click **Import...**.
4. Select the `certs/ca.crt` file generated in your ShadowSchema directory.
5. Check **Trust this CA to identify websites** and click **OK**.

Now, traffic from Firefox Developer Edition will seamlessly route through ShadowSchema!

### 🔑 Trust Provisioning (System-Wide)

Upon initial launch, ShadowSchema will forge a fresh RSA keypair and self-signed Certificate Authority within the `certs/` directory. 
To achieve seamless HTTPS interception without triggering `ERR_CERT_AUTHORITY_INVALID` anomalies:
1. Locate `certs/ca.crt`.
2. Inject it into your operating system's root trust store or browser authority list.

## 📡 Spec Extraction

While the proxy actively intercepts and builds the map, you can extract the live OpenAPI specification via the dashboard's "Export JSON" button, or hit the extraction node directly:

```bash
# Pull the live schema payload
curl -s http://localhost:38081/export-map
```

Alternatively, dispatch a `Ctrl+C` interrupt. ShadowSchema will catch the signal, perform a graceful shutdown, and dump the final footprint directly to `openapi.json` in your current working directory.

### Export API Endpoints

The background export server on `:38081` powers the dashboard and CLI tooling:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/export-map` | GET | Live OpenAPI spec (JSON or `?format=yaml`) |
| `/vault` | GET | Captured auth credentials |
| `/discovered` | GET | Out-of-scope domains seen via CONNECT |
| `/sessions` | GET, POST | List or create recon sessions |
| `/sessions/switch` | POST | Activate a saved session |
| `/sessions/delete` | POST | Delete a session |
| `/sessions/add-target` | POST | Append a domain to the active target list |
| `/generate-sdk` | POST | Generate a Python, TypeScript, Go, or Rust SDK zip |

## 🧪 Testing

ShadowSchema ships with unit and integration tests across the proxy engine, export API, and schema inference pipeline.

```bash
# Full suite (uses in-memory SQLite via pure-Go driver)
go test ./...

# Static analysis (enforced in CI)
go vet ./...
gosec ./...
```

Coverage is strongest in `internal/router` (100%), `internal/spec` (~73%), and `main` proxy integration tests. SDK generation tests skip automatically when `npx` is unavailable.

## 🤝 Contributing

We welcome pull requests! See `CONTRIBUTING.md` for our guidelines, and `THIRDPARTY.md` for information on our open-source dependencies. Please ensure you run `go vet ./...` and `gosec ./...` before submitting your changes.

## ⚖️ Legal Disclaimer

**For Educational and Authorized Use Only.** ShadowSchema is designed exclusively for security research, systems architecture analysis, and debugging on networks and APIs where you have explicit authorization to do so. The author assumes no liability and is not responsible for any misuse, damage, or unauthorized access caused by this software. Use responsibly and abide by all applicable local and international laws.

## 🛋️ Origin Story

*Why build this?* I suffered a back injury a while back, which means I now spend a lot of time laying around with my laptop. Figuring out undocumented APIs from the couch sounded like a fun way to pass the time, so here we are!

---
<div align="center">
<i>"Visibility is the first step to understanding."</i>
</div>
