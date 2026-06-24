import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it, mock } from "node:test";
import { ShadowSchemaClient } from "../src/client.js";

const config = {
  exportUrl: "http://shadowschema.test:38081",
  proxyUrl: "http://127.0.0.1:38080",
};

describe("ShadowSchemaClient", () => {
  let originalFetch: typeof fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    mock.restoreAll();
  });

  it("loads sessions from /sessions", async () => {
    globalThis.fetch = mock.fn(async () =>
      Response.json([
        {
          id: 1,
          name: "Demo",
          target: "api.example.com",
          ignore_rules: "",
          updated_at: "2026-01-01T00:00:00Z",
        },
      ]),
    ) as typeof fetch;

    const client = new ShadowSchemaClient(config);
    const sessions = await client.listSessions();
    assert.equal(sessions.length, 1);
    assert.equal(sessions[0]?.name, "Demo");
  });

  it("creates a session via POST /sessions", async () => {
    const fetchMock = mock.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      assert.equal(String(input), `${config.exportUrl}/sessions`);
      assert.equal(init?.method, "POST");
      return new Response(null, { status: 200 });
    });
    globalThis.fetch = fetchMock as typeof fetch;

    const client = new ShadowSchemaClient(config);
    await client.createSession("Test", "api.example.com");
    assert.equal(fetchMock.mock.calls.length, 1);
  });

  it("reports health from /health", async () => {
    globalThis.fetch = mock.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/health")) {
        return Response.json({
          status: "ok",
          session_id: 1,
          session_name: "Demo",
          target: "api.example.com",
          endpoint_count: 2,
          active_session: true,
          session_updated_at: "2026-01-01T00:00:00Z",
        });
      }
      throw new Error(`unexpected fetch: ${url}`);
    }) as typeof fetch;

    const client = new ShadowSchemaClient(config);
    const health = await client.healthCheck();
    assert.equal(health.ok, true);
    assert.equal(health.endpointCount, 2);
    assert.equal(health.sessionName, "Demo");
  });

  it("throws when export API is unreachable", async () => {
    globalThis.fetch = mock.fn(async () => new Response("nope", { status: 503 })) as typeof fetch;

    const client = new ShadowSchemaClient(config);
    await assert.rejects(() => client.getSpecJson(), /GET \/export-map failed/);
  });
});