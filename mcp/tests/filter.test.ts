import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { OpenApiDocument } from "../src/client.js";
import {
  filterSpecByPathPrefix,
  getEndpointDetail,
  listEndpointSummaries,
} from "../src/utils/filter.js";

const sampleSpec: OpenApiDocument = {
  openapi: "3.0.0",
  paths: {
    "/api/users": {
      get: {
        "x-last-payload": { id: 1 },
        responses: { "200": { description: "ok" } },
      },
    },
    "/api/items/{id}": {
      post: {
        responses: { "201": { description: "created" } },
      },
    },
    "/ws/chat/{uuid}": {
      get: {
        "x-websocket": true,
        "x-websocket-message-schema-in": { type: "object" },
      },
    },
    "/health": {
      get: {
        responses: { "200": { description: "ok" } },
      },
    },
  },
};

describe("listEndpointSummaries", () => {
  it("returns compact endpoint index", () => {
    const endpoints = listEndpointSummaries(sampleSpec);
    assert.equal(endpoints.length, 4);
    assert.deepEqual(endpoints[0], {
      path: "/api/items/{id}",
      methods: ["POST"],
      websocket: false,
      hasLastPayload: false,
      hasWebsocketSchemas: false,
    });
  });

  it("filters by path prefix", () => {
    const endpoints = listEndpointSummaries(sampleSpec, "/api");
    assert.equal(endpoints.length, 2);
    assert.ok(endpoints.every((entry) => entry.path.startsWith("/api")));
  });
});

describe("filterSpecByPathPrefix", () => {
  it("returns only matching paths", () => {
    const filtered = filterSpecByPathPrefix(sampleSpec, "/api");
    const paths = filtered.paths as Record<string, unknown>;
    assert.equal(Object.keys(paths).length, 2);
    assert.ok(paths["/api/users"]);
    assert.ok(paths["/api/items/{id}"]);
  });
});

describe("getEndpointDetail", () => {
  it("returns operations for a known path", () => {
    const detail = getEndpointDetail(sampleSpec, "/api/users");
    assert.ok(detail);
    assert.equal(detail.path, "/api/users");
    assert.deepEqual(detail.methods, ["GET"]);
    assert.ok((detail.operations as Record<string, unknown>).get);
  });

  it("returns null for unknown path", () => {
    assert.equal(getEndpointDetail(sampleSpec, "/missing"), null);
  });
});