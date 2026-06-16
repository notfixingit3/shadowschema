<div align="center">
  <pre>
   _____ _               _               _____      _                          
  / ____| |             | |             / ____|    | |                         
 | (___ | |__   __ _  __| | _____      | (___   ___| |__   ___ _ __ ___   __ _ 
  \___ \| '_ \ / _` |/ _` |/ _ \ \ /\ / /\___ \ / __| '_ \ / _ \ '_ ` _ \ / _` |
  ____) | | | | (_| | (_| | (_) \ V  V / ____) | (__| | | |  __/ | | | | | (_| |
 |_____/|_| |_|\__,_|\__,_|\___/ \_/\_/ |_____/ \___|_| |_|\___|_| |_| |_|\__,_|
  </pre>
  <p><b>Advanced API MITM Mapping & Reconnaissance Framework</b></p>
</div>

---

## 👁️ Overview

**ShadowSchema** is a specialized, clandestine Man-in-the-Middle (MITM) proxy engineered in Go. Designed for advanced API reconnaissance, it silently intercepts target HTTP/HTTPS telemetry, deduces underlying JSON payloads, and programmatically reconstructs evolving OpenAPI 3.0 specifications on the fly.

Built for red teamers, security researchers, and systems architects who need to map undocumented endpoints in real-time.

## ⚡ Core Capabilities

- **Deep TLS Inspection:** Deploys a dynamically generated local Certificate Authority (CA) on startup, effortlessly bypassing HTTPS encryption to inspect application layers.
- **Heuristic Schema Inference:** Parses intercepted JSON telemetry recursively, performing automated type detection and bridging schema mutations iteratively.
- **Intelligent Routing Deduplication:** Aggregates variable routes through regex-driven pattern matching (UUIDs, IDs, Timestamps), drastically reducing map noise.
- **Ghost Logging:** Maintains an ultra-clean, noise-free terminal footprint with aligned status maps and disabled reverse DNS lookups.
- **Asynchronous Extraction:** Exfiltrates the mapped OpenAPI state gracefully upon system interrupt (`SIGTERM`) or via a clandestine, background API extraction node.

## 🛠️ Infrastructure Requirements

- **Runtime:** Go 1.18+
- **Privileges:** Root CA (`certs/ca.crt`) installation capabilities to satisfy client-side SSL validation constraints.

## 🚀 Deployment

Initiate the proxy engine and specify the target perimeter you wish to map. 

```bash
# Initiate mapping against the target domain
go run main.go --target=example.com
```

### Command Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--target` | `example.com` | The target domain perimeter to monitor and intercept. |
| `--port` | `:38080` | Local port bound for the MITM proxy listener. |
| `--export-port` | `:38081` | Local port bound for the background OpenAPI extraction server. |

### 🔑 Trust Provisioning

Upon initial launch, ShadowSchema will forge a fresh RSA keypair and self-signed Certificate Authority within the `certs/` directory. 
To achieve seamless HTTPS interception without triggering `ERR_CERT_AUTHORITY_INVALID` anomalies:
1. Locate `certs/ca.crt`.
2. Inject it into your operating system's root trust store or browser authority list.

## 📡 Spec Extraction

While the proxy actively intercepts and builds the map, you can extract the live OpenAPI specification via the extraction node:

```bash
# Pull the live schema payload
curl -s http://localhost:38081/export-map
```

Alternatively, dispatch a `Ctrl+C` interrupt. ShadowSchema will catch the signal, perform a graceful shutdown, and dump the final footprint directly to `openapi.json` in your current working directory.

## ⚖️ Legal Disclaimer

**For Educational and Authorized Use Only.** ShadowSchema is designed exclusively for security research, systems architecture analysis, and debugging on networks and APIs where you have explicit authorization to do so. The author assumes no liability and is not responsible for any misuse, damage, or unauthorized access caused by this software. Use responsibly and abide by all applicable local and international laws.

## 🛋️ Origin Story

*Why build this?* I suffered a back injury a while back, which means I now spend a lot of time laying around with my laptop. Figuring out undocumented APIs from the couch sounded like a fun way to pass the time, so here we are!

---
<div align="center">
<i>"Visibility is the first step to understanding."</i>
</div>
