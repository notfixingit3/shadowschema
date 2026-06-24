import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { z } from "zod";
import { exploreTarget } from "./crawl/explore.js";
import { ShadowSchemaClient } from "./client.js";
import { loadConfig } from "./config.js";
import { proxySetupMarkdown } from "./resources/setup.js";
import { generatePythonReplayScript } from "./utils/replay.js";
import { getEndpointDetail, listEndpointSummaries } from "./utils/filter.js";
import { waitForEndpoints } from "./utils/poll.js";
import { isTcpPortOpen, parseProxyUrl } from "./utils/proxy-check.js";
import { SpecSnapshotStore } from "./utils/snapshot.js";
import { INFERRED_SCHEMA_NOTE, LEGAL_NOTE, VAULT_SECURITY_NOTE } from "./utils/security.js";

function textResult(text: string) {
  return { content: [{ type: "text" as const, text }] };
}

function jsonResult(value: unknown) {
  return textResult(JSON.stringify(value, null, 2));
}

function toolError(message: string) {
  return {
    content: [{ type: "text" as const, text: message }],
    isError: true as const,
  };
}

export function createServer(
  client: ShadowSchemaClient,
  snapshots: SpecSnapshotStore = new SpecSnapshotStore(),
): McpServer {
  const server = new McpServer({
    name: "shadowschema-mcp",
    version: "0.3.0",
  });

  server.registerTool(
    "shadowschema_health",
    {
      description: `Check ShadowSchema export API connectivity. ${INFERRED_SCHEMA_NOTE}`,
      inputSchema: z.object({}),
    },
    async () => {
      try {
        const health = await client.healthCheck();
        return jsonResult({
          status: "ok",
          export_url: client.exportUrl,
          proxy_url: client.proxyUrl,
          ...health,
        });
      } catch (error) {
        return toolError(
          `ShadowSchema export API unreachable at ${client.exportUrl}. Start with: docker compose up -d\n\n${String(error)}`,
        );
      }
    },
  );

  server.registerTool(
    "shadowschema_list_sessions",
    {
      description: "List saved ShadowSchema recon sessions.",
      inputSchema: z.object({}),
    },
    async () => {
      try {
        return jsonResult(await client.listSessions());
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_create_session",
    {
      description: "Create a new recon session for a target API hostname.",
      inputSchema: z.object({
        name: z.string().describe("Session display name"),
        target: z.string().describe("Target hostname, e.g. api.example.com"),
        ignore_rules: z
          .string()
          .optional()
          .describe("Optional regex ignore rules for static assets"),
      }),
    },
    async ({ name, target, ignore_rules }) => {
      try {
        await client.createSession(name, target, ignore_rules ?? "");
        return jsonResult({ ok: true, name, target });
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_switch_session",
    {
      description: "Activate a saved recon session by ID.",
      inputSchema: z.object({
        id: z.number().int().positive().describe("Session ID from list_sessions"),
      }),
    },
    async ({ id }) => {
      try {
        await client.switchSession(id);
        return jsonResult({ ok: true, active_session_id: id });
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_get_spec",
    {
      description: `Fetch the live OpenAPI spec. ${INFERRED_SCHEMA_NOTE}`,
      inputSchema: z.object({
        format: z.enum(["json", "yaml"]).optional().default("json"),
        path_prefix: z
          .string()
          .optional()
          .describe("Return only paths starting with this prefix, e.g. /api/v1"),
        session_id: z
          .number()
          .int()
          .positive()
          .optional()
          .describe("Read spec for a non-active session without switching"),
      }),
    },
    async ({ format, path_prefix, session_id }) => {
      try {
        if (format === "yaml" && !path_prefix && session_id === undefined) {
          return textResult(await client.getSpec("yaml"));
        }

        const spec = await client.getSpecJson({ pathPrefix: path_prefix, sessionId: session_id });
        if (format === "yaml") {
          return textResult(await client.getSpec("yaml", { pathPrefix: path_prefix, sessionId: session_id }));
        }
        return textResult(JSON.stringify(spec, null, 2));
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_list_endpoints",
    {
      description: `List discovered endpoints as a compact index. ${INFERRED_SCHEMA_NOTE}`,
      inputSchema: z.object({
        path_prefix: z.string().optional().describe("Filter paths by prefix"),
        session_id: z.number().int().positive().optional(),
      }),
    },
    async ({ path_prefix, session_id }) => {
      try {
        const index = await client.listEndpointsIndex(path_prefix, session_id);
        return jsonResult(index);
      } catch (error) {
        try {
          const spec = await client.getSpecJson({ pathPrefix: path_prefix, sessionId: session_id });
          const endpoints = listEndpointSummaries(spec, path_prefix);
          return jsonResult({ count: endpoints.length, endpoints });
        } catch (fallbackError) {
          return toolError(String(fallbackError));
        }
      }
    },
  );

  server.registerTool(
    "shadowschema_get_endpoint",
    {
      description: `Get schema and extensions for one endpoint path. ${INFERRED_SCHEMA_NOTE}`,
      inputSchema: z.object({
        path: z.string().describe("OpenAPI path, e.g. /api/v1/users/{id}"),
        session_id: z.number().int().positive().optional(),
      }),
    },
    async ({ path, session_id }) => {
      try {
        const detail = await client.getEndpointFromApi(path, session_id);
        return jsonResult(detail);
      } catch (error) {
        try {
          const spec = await client.getSpecJson({ sessionId: session_id });
          const detail = getEndpointDetail(spec, path);
          if (!detail) {
            return toolError(`Endpoint not found: ${path}`);
          }
          return jsonResult(detail);
        } catch (fallbackError) {
          return toolError(String(fallbackError));
        }
      }
    },
  );

  server.registerTool(
    "shadowschema_get_vault",
    {
      description: `Get captured auth credentials from the active session. ${VAULT_SECURITY_NOTE}`,
      inputSchema: z.object({}),
    },
    async () => {
      try {
        const credentials = await client.getVault();
        return jsonResult({
          warning: VAULT_SECURITY_NOTE,
          count: credentials.length,
          credentials,
        });
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_list_discovered_domains",
    {
      description:
        "List out-of-scope domains seen via CONNECT that may need to be added to the target list.",
      inputSchema: z.object({}),
    },
    async () => {
      try {
        const domains = await client.listDiscoveredDomains();
        return jsonResult({ count: domains.length, domains });
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_add_target_domain",
    {
      description: "Append a domain to the active session target list (shadow domains).",
      inputSchema: z.object({
        domain: z.string().describe("Hostname to add, e.g. cdn.example.com"),
      }),
    },
    async ({ domain }) => {
      try {
        await client.addTargetDomain(domain);
        return jsonResult({ ok: true, domain });
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_generate_sdk",
    {
      description: `Generate a typed client SDK zip from the live OpenAPI spec. ${INFERRED_SCHEMA_NOTE}`,
      inputSchema: z.object({
        language: z
          .enum(["python", "typescript-fetch", "go", "rust"])
          .optional()
          .default("typescript-fetch"),
        output: z
          .enum(["base64", "path"])
          .optional()
          .default("base64")
          .describe("Return base64 zip data or write to a file path"),
        write_path: z
          .string()
          .optional()
          .describe("File path when output=path (defaults to temp dir)"),
      }),
    },
    async ({ language, output, write_path }) => {
      try {
        const { data, excluded } = await client.generateSdk(language);
        const filename = `${language}_sdk.zip`;

        if (output === "path") {
          const targetPath = write_path ?? join(tmpdir(), `shadowschema-${filename}`);
          await writeFile(targetPath, data);
          return jsonResult({
            ok: true,
            language,
            path: targetPath,
            size_bytes: data.length,
            websocket_endpoints_excluded: excluded,
          });
        }

        return jsonResult({
          ok: true,
          language,
          filename,
          size_bytes: data.length,
          websocket_endpoints_excluded: excluded,
          base64: data.toString("base64"),
        });
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_get_setup_status",
    {
      description:
        "Report export API and MITM proxy reachability plus active session metadata.",
      inputSchema: z.object({
        session_id: z.number().int().positive().optional(),
      }),
    },
    async ({ session_id }) => {
      try {
        const { host, port } = parseProxyUrl(client.proxyUrl);
        const [health, proxyReachable] = await Promise.all([
          client.getHealth(session_id),
          isTcpPortOpen(host, port),
        ]);

        return jsonResult({
          export_api: {
            reachable: true,
            url: client.exportUrl,
          },
          proxy: {
            reachable: proxyReachable,
            url: client.proxyUrl,
            host,
            port,
          },
          session: {
            id: health.session_id,
            name: health.session_name,
            target: health.target,
            active: health.active_session,
            updated_at: health.session_updated_at,
          },
          endpoint_count: health.endpoint_count,
        });
      } catch (error) {
        const { host, port } = parseProxyUrl(client.proxyUrl);
        const proxyReachable = await isTcpPortOpen(host, port);
        return jsonResult({
          export_api: {
            reachable: false,
            url: client.exportUrl,
            error: String(error),
          },
          proxy: {
            reachable: proxyReachable,
            url: client.proxyUrl,
            host,
            port,
          },
        });
      }
    },
  );

  server.registerTool(
    "shadowschema_wait_for_endpoints",
    {
      description: `Poll until endpoint coverage meets a threshold. ${INFERRED_SCHEMA_NOTE}`,
      inputSchema: z.object({
        min_count: z.number().int().positive().optional().default(1),
        path_prefix: z.string().optional(),
        timeout_ms: z.number().int().positive().optional().default(60_000),
        interval_ms: z.number().int().positive().optional().default(2_000),
        session_id: z.number().int().positive().optional(),
      }),
    },
    async ({ min_count, path_prefix, timeout_ms, interval_ms, session_id }) => {
      try {
        const result = await waitForEndpoints(client, {
          minCount: min_count,
          pathPrefix: path_prefix,
          timeoutMs: timeout_ms,
          intervalMs: interval_ms,
          sessionId: session_id,
        });
        return jsonResult(result);
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_spec_diff",
    {
      description: `Return newly discovered endpoint paths since the previous call. ${INFERRED_SCHEMA_NOTE}`,
      inputSchema: z.object({
        session_id: z.number().int().positive().optional(),
        path_prefix: z.string().optional(),
        reset: z
          .boolean()
          .optional()
          .default(false)
          .describe("Clear the stored snapshot before diffing"),
      }),
    },
    async ({ session_id, path_prefix, reset }) => {
      try {
        if (reset) {
          snapshots.reset(session_id);
        }

        const index = await client.listEndpointsIndex(path_prefix, session_id);
        const paths = index.endpoints.map((entry) => entry.path);
        const diff = snapshots.diff(session_id, paths);
        return jsonResult({
          ...diff,
          endpoints: index.endpoints.filter((entry) => diff.new_paths.includes(entry.path)),
        });
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_explore_target",
    {
      description: `Crawl a target through the ShadowSchema MITM proxy to generate API traffic. ${INFERRED_SCHEMA_NOTE} ${LEGAL_NOTE}`,
      inputSchema: z.object({
        start_url: z.string().url().describe("First page to open, e.g. https://app.example.com/"),
        max_pages: z.number().int().positive().optional().default(10),
        max_depth: z.number().int().nonnegative().optional().default(2),
        wait_ms: z.number().int().positive().optional().default(1500),
        session_id: z.number().int().positive().optional(),
        storage_state_path: z
          .string()
          .optional()
          .describe("Playwright storage state JSON for authenticated crawls"),
        ignore_https_errors: z.boolean().optional().default(true),
      }),
    },
    async ({
      start_url,
      max_pages,
      max_depth,
      wait_ms,
      session_id,
      storage_state_path,
      ignore_https_errors,
    }) => {
      try {
        const result = await exploreTarget(client, {
          startUrl: start_url,
          maxPages: max_pages,
          maxDepth: max_depth,
          waitMs: wait_ms,
          proxyUrl: client.proxyUrl,
          storageStatePath: storage_state_path,
          ignoreHTTPSErrors: ignore_https_errors,
          sessionId: session_id,
        });
        return jsonResult(result);
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerTool(
    "shadowschema_export_replay_script",
    {
      description: `Generate a Python requests replay script for one endpoint using vault auth and last-seen payload. ${VAULT_SECURITY_NOTE}`,
      inputSchema: z.object({
        path: z.string().describe("OpenAPI path, e.g. /api/v1/users"),
        method: z.string().describe("HTTP method, e.g. GET or POST"),
        session_id: z.number().int().positive().optional(),
      }),
    },
    async ({ path, method, session_id }) => {
      try {
        const script = await client.exportReplay(path, method, session_id);
        return textResult(script);
      } catch (error) {
        try {
          const [detail, vault, health] = await Promise.all([
            client.getEndpointFromApi(path, session_id),
            client.getVault(),
            client.getHealth(session_id),
          ]);

          const operations = detail.operations as Record<string, Record<string, unknown>>;
          const operation = operations[method.toLowerCase()];
          if (!operation) {
            return toolError(`Method not found for endpoint: ${method} ${path}`);
          }

          const script = generatePythonReplayScript({
            path,
            method,
            operation,
            target: health.target,
            vault,
          });
          return textResult(script);
        } catch (fallbackError) {
          return toolError(String(fallbackError));
        }
      }
    },
  );

  server.registerTool(
    "shadowschema_get_ca_cert",
    {
      description: "Download the ShadowSchema MITM root CA certificate (PEM).",
      inputSchema: z.object({}),
    },
    async () => {
      try {
        const cert = await client.getCaCert();
        return textResult(cert);
      } catch (error) {
        return toolError(String(error));
      }
    },
  );

  server.registerResource(
    "openapi-spec",
    "shadowschema://spec/openapi.json",
    {
      description: `Live inferred OpenAPI spec. ${INFERRED_SCHEMA_NOTE}`,
      mimeType: "application/json",
    },
    async () => ({
      contents: [
        {
          uri: "shadowschema://spec/openapi.json",
          mimeType: "application/json",
          text: JSON.stringify(await client.getSpecJson(), null, 2),
        },
      ],
    }),
  );

  server.registerResource(
    "proxy-setup",
    "shadowschema://setup/proxy.md",
    {
      description: "Guide for routing traffic through the ShadowSchema MITM proxy.",
      mimeType: "text/markdown",
    },
    async () => ({
      contents: [
        {
          uri: "shadowschema://setup/proxy.md",
          mimeType: "text/markdown",
          text: proxySetupMarkdown(loadConfig()),
        },
      ],
    }),
  );

  return server;
}

