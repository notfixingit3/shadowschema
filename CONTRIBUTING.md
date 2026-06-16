# Contributing to ShadowSchema

First off, thank you for considering contributing to ShadowSchema! It's people like you that make the open-source community such an amazing place to learn, inspire, and create.

### How Can I Contribute?

* **Report Bugs:** If you find a bug in the proxy engine or dashboard, please open an issue describing the bug, including steps to reproduce.
* **Suggest Enhancements:** Have an idea to make mapping faster, stealthier, or more accurate? Open an issue outlining your proposed feature!
* **Submit Pull Requests:** 
  1. Fork the repo and create your branch from `dev`.
  2. If you've added code that should be tested, add Go unit tests.
  3. Ensure the test suite passes with CGO enabled: `CGO_ENABLED=1 go test ./...`.
  4. Run static checks before opening a PR: `go vet ./...` and `gosec ./...` (CI enforces both).
  5. Make sure your code aligns with the existing style.
  6. Issue that pull request!

### Development Setup

To get started locally:
1. Clone the repository.
2. Run `go run main.go` to start the backend proxy.
3. In a separate terminal, navigate to `dashboard/` and run `npm install && npm run dev`.

### Testing Guidelines

- **Proxy changes:** Add or extend tests in `main_test.go`. The proxy pipeline is exposed via `newProxyServer()` for httptest integration.
- **Export API / spec logic:** Add tests in `internal/spec/`. Use `newTestSpecManager(t, target)` from `testutil_test.go` so each test gets an isolated SQLite database in a temp directory.
- **SDK generation:** Tests that call `npx` should skip gracefully when the tool or network is unavailable (`t.Skip`).
- **Before a release:** Update `CHANGELOG.md`, bump the version in `dashboard/index.html`, and tag with `v*.*.*` to trigger the GitHub release workflow.

### Code of Conduct

Please note that this project is released with a Contributor Code of Conduct. By participating in this project you agree to abide by its terms. Let's build something awesome together!
