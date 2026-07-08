import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { generatePythonReplayScript } from "../src/utils/replay.js";

describe("generatePythonReplayScript", () => {
  it("includes vault headers and request body payload for POST", () => {
    const script = generatePythonReplayScript({
      path: "/api/items",
      method: "POST",
      operation: {
        "x-last-payload": { id: 42, created: true },
        "x-last-request-body": { name: "widget" },
      },
      target: "api.example.com",
      vault: [{ header_name: "Authorization", token_value: "Bearer x", first_seen: "" }],
    });

    assert.match(script, /Auth headers auto-injected/);
    assert.match(script, /Authorization/);
    assert.match(script, /payload =/);
    assert.match(script, /widget/);
    assert.doesNotMatch(script, /created/);
    assert.match(script, /https:\/\/api\.example\.com\/api\/items/);
  });

  it("does not use response payload as request body", () => {
    const script = generatePythonReplayScript({
      path: "/api/items",
      method: "POST",
      operation: { "x-last-payload": { id: 42 } },
      target: "api.example.com",
      vault: [],
    });
    assert.match(script, /No request body captured/);
    assert.doesNotMatch(script, /payload =/);
  });
});