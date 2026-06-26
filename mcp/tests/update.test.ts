import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { tryAutoUpdate } from "../src/utils/update.js";

describe("tryAutoUpdate", () => {
  it("returns without error when auto-update is disabled", async () => {
    await assert.doesNotReject(() => tryAutoUpdate("/tmp", false));
  });

  it("returns without error outside a git repository", async () => {
    await assert.doesNotReject(() => tryAutoUpdate("/tmp", true));
  });
});