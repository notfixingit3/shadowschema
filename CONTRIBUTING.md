# Contributing to ShadowSchema

First off, thank you for considering contributing to ShadowSchema! It's people like you that make the open-source community such an amazing place to learn, inspire, and create.

### How Can I Contribute?

* **Report Bugs:** If you find a bug in the proxy engine or dashboard, please open an issue describing the bug, including steps to reproduce.
* **Suggest Enhancements:** Have an idea to make mapping faster, stealthier, or more accurate? Open an issue outlining your proposed feature!
* **Submit Pull Requests:** 
  1. Fork the repo and create your branch from `dev`.
  2. If you've added code that should be tested, add Go unit tests.
  3. Ensure the test suite passes: `go test ./...`.
  4. Run static checks before opening a PR: `go vet ./...` and `gosec ./...` (CI enforces both).
  5. Make sure your code aligns with the existing style.
  6. Issue that pull request!

### Development Setup

**Contributors** run the Go and Node dev toolchains directly. **End users** should use the pre-built Docker images (`docker compose up` or `deploy/preview/`) — see `README.md` (Quick Start and **Choosing stable vs beta**).

To get started locally:
1. Clone the repository.
2. Run `go run main.go` to start the backend proxy (uses SQLite at `./shadowschema.db` when `DATABASE_URL` is unset).
3. In a separate terminal, navigate to `dashboard/` and run `npm install && npm run dev`.

The Vite dev server proxies export API routes to `:38081` so the dashboard works at `http://localhost:5173` without extra configuration.

To smoke-test production images before pushing:

```bash
docker build -t shadowschema:local .
docker build -f Dockerfile.dashboard -t shadowschema-dashboard:local .
SHADOWSCHEMA_IMAGE=shadowschema:local SHADOWSCHEMA_DASHBOARD_IMAGE=shadowschema-dashboard:local docker compose up -d
```

### Testing Guidelines

- **Proxy changes:** Add or extend tests in `main_test.go`. The proxy pipeline is exposed via `newProxyServer()` for httptest integration.
- **Export API / spec logic:** Add tests in `internal/spec/`. Use `newTestSpecManager(t, target)` from `testutil_test.go` so each test gets an isolated SQLite database in a temp directory.
- **SDK generation:** Tests that call `npx` should skip gracefully when the tool or network is unavailable (`t.Skip`).
- **Before a release:** Update `CHANGELOG.md`, bump the version in `dashboard/index.html`, and tag with `v*.*.*` to trigger the GitHub release workflow.
- **Docker images:** `.github/workflows/docker.yml` builds and publishes proxy + dashboard images to GHCR on every push to `main` or `dev` (and on version tags). `dev` gets `:beta` and `:dev`; `main` gets `:latest` and `:main`; git tags publish `:vX.Y.Z`. Document tag choices in `README.md` when behavior changes.
- **preview preview:** Stack lives at `/opt/stacks/shadowschema_preview` on `notfixingit`. Sync `deploy/preview/` (compose, nginx configs, `.env.example` — not the git repo), ensure `.env` exists with `POSTGRES_PASSWORD`, then `docker compose pull && docker compose up -d`. Requires `postgres`, `proxy`, `dashboard`, and `nginx` services; proxy needs `DATABASE_URL` (set automatically by compose).

### Code of Conduct

Please note that this project is released with a Contributor Code of Conduct. By participating in this project you agree to abide by its terms. Let's build something awesome together!
