import assert from "node:assert/strict";
import { describe, it, mock } from "node:test";
import type { ShadowSchemaClient } from "../src/client.js";
import { waitForEndpoints } from "../src/utils/poll.js";

describe("waitForEndpoints", () => {
  it("returns when min_count is satisfied", async () => {
    let calls = 0;
    const client = {
      listEndpointsIndex: async () => {
        calls += 1;
        return {
          count: calls,
          session_id: 1,
          endpoints:
            calls >= 2
              ? [{ path: "/api/ping", methods: ["GET"], has_payload: true, websocket: false }]
              : [],
        };
      },
    } as unknown as ShadowSchemaClient;

    const result = await waitForEndpoints(client, {
      minCount: 1,
      intervalMs: 10,
      timeoutMs: 500,
    });

    assert.equal(result.satisfied, true);
    assert.equal(result.matchingCount, 1);
    assert.ok(result.waitedMs >= 0);
  });
});