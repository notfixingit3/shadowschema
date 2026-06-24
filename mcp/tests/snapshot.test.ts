import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { SpecSnapshotStore } from "../src/utils/snapshot.js";

describe("SpecSnapshotStore", () => {
  it("tracks new and removed paths between diffs", () => {
    const store = new SpecSnapshotStore();

    const first = store.diff(undefined, ["/api/a", "/api/b"]);
    assert.deepEqual(first.new_paths, ["/api/a", "/api/b"]);
    assert.deepEqual(first.removed_paths, []);

    const second = store.diff(undefined, ["/api/b", "/api/c"]);
    assert.deepEqual(second.new_paths, ["/api/c"]);
    assert.deepEqual(second.removed_paths, ["/api/a"]);
  });

  it("scopes snapshots by session id", () => {
    const store = new SpecSnapshotStore();
    store.diff(1, ["/one"]);
    const other = store.diff(2, ["/two"]);
    assert.deepEqual(other.new_paths, ["/two"]);
  });
});