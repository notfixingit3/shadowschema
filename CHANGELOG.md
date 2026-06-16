# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- **Core Engine:** Automated HTTP/HTTPS proxy using `elazarl/goproxy`.
- **Security:** Dynamic CA generation with `crypto/x509` and automatic trust bridging.
- **Routing:** Basic API path deduplication using regular expressions (`/{uuid}`, `/{id}`, `/{year}`).
- **Schema Mapping:** JSON schema inference engine capable of automated type detection and recursive schema evolution.
- **Data Persistence:** Added SQLite integration to safely persist OpenAPI specifications across restarts (`shadowschema.db`).
- **Telemetry:** Proxy now intercepts and maps URL query parameters and custom HTTP headers automatically.
- **UI:** Introduced a real-time, glassmorphism-styled Vite + Vanilla JS web dashboard for visualizing intercepted data dynamically.
- **Exporting:** OpenAPI specification management with background export server running on `:38081`. Now featuring CORS support for the web dashboard.
- **UX:** Clean logging with status code mappings and disabled reverse DNS lookups.
- **System:** Smart availability port checking before binding.

### Changed
- Moved default proxy port to `:38080` and export port to `:38081` to prevent standard port collisions.
- Added a Scooby-Doo git commit message hook.
