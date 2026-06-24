import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { extractSameOriginLinks, normalizeUrl, shouldSkipUrl } from "../src/crawl/links.js";

describe("crawl link helpers", () => {
  it("keeps same-origin links and drops static assets", () => {
    const page = new URL("https://app.example.com/dashboard");
    const links = extractSameOriginLinks(page, [
      "/settings",
      "https://app.example.com/api/docs",
      "https://cdn.example.com/lib.js",
      "https://app.example.com/logo.png",
      "mailto:admin@example.com",
    ]);

    assert.deepEqual(links, [
      "https://app.example.com/api/docs",
      "https://app.example.com/settings",
    ]);
  });

  it("normalizes away hash fragments", () => {
    const url = new URL("https://app.example.com/page#section");
    assert.equal(normalizeUrl(url), "https://app.example.com/page");
  });

  it("skips non-http protocols", () => {
    assert.equal(shouldSkipUrl(new URL("mailto:test@example.com")), true);
  });
});