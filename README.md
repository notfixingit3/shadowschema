# ShadowSchema

ShadowSchema is an API Man-in-the-Middle (MITM) Mapper written in Go. It intercepts HTTP and HTTPS traffic for a target domain, infers OpenAPI schemas from the JSON responses, deduplicates similar paths, and dynamically generates an OpenAPI v3.0 specification.

## Features

- **Automated MITM Proxy**: Intercepts secure traffic seamlessly using `elazarl/goproxy`.
- **Dynamic CA Generation**: Auto-generates local Certificate Authority (CA) certificates on startup to enable HTTPS interception without manual cert creation.
- **Path Deduplication**: Normalizes variable paths (UUIDs, Integers, Years) automatically.
- **Schema Inference**: Parses JSON responses on the fly and builds evolving OpenAPI structures.
- **Export Capabilities**: Dumps the spec to `openapi.json` gracefully on shutdown or via a local background server on demand.
- **Clean Logging**: Simple, snappy, and cleanly aligned console output.

## Prerequisites

- Go 1.18+
- The generated root CA (`certs/ca.crt`) must be trusted by your operating system or browser for full HTTPS interception.

## Usage

Start the proxy by passing the `--target` flag:

```bash
go run main.go --target=example.com
```

### Available Flags:

- `--target` (string): The target domain to intercept and map. Only this domain's traffic will be evaluated (default is `example.com`).
- `--port` (string): Port for the MITM proxy. Default is `:38080`.
- `--export-port` (string): Port for the OpenAPI export server. Default is `:38081`.

### Trusting the CA

On the first run, ShadowSchema will generate a `certs/` folder with a new CA certificate (`ca.crt`) and private key. Install and trust `certs/ca.crt` on your system to avoid browser security warnings.

## Exporting the Spec

While the proxy is running, you can export the currently mapped schema by hitting the export endpoint:

```bash
curl http://localhost:38081/export-map
```

When you stop the server gracefully (Ctrl+C), the schema will also automatically be written to `openapi.json` in the root directory.
