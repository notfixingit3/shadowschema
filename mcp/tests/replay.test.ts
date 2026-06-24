import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { generatePythonReplayScript } from "../src/utils/replay.js";

describe("generatePythonReplayScript", () => {
  it("includes vault headers and payload for POST", () => {
    const script = generatePythonReplayScript({
      path: "/api/items",
      method: "POST",
      operation: { "x-last-payload": { id: 42 } },
      target: "api.example.com",
      vault: [{ header_name: "Authorization", token_value: "Bearer x", first_seen: "" }],
    });

    assert.match(script, /Auth headers auto-injected/);
    assert.match(script, /Authorization/);
    assert.match(script, /payload =/);
    assert.match(script, /https:\/\/api\.example\.com\/api\/items/);
  });
});