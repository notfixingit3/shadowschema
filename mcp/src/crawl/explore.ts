import type { ShadowSchemaClient } from "../client.js";
import { extractSameOriginLinks, normalizeUrl, shouldSkipUrl } from "./links.js";
import { sleep } from "../utils/sleep.js";

export interface ExploreTargetOptions {
  startUrl: string;
  maxPages?: number;
  maxDepth?: number;
  waitMs?: number;
  proxyUrl: string;
  storageStatePath?: string;
  ignoreHTTPSErrors?: boolean;
  sessionId?: number;
}

export interface ExploreTargetResult {
  start_url: string;
  pages_visited: string[];
  domains_hit: string[];
  endpoint_count_before: number;
  endpoint_count_after: number;
  endpoint_delta: number;
  max_pages: number;
  max_depth: number;
  errors: string[];
}

async function loadPlaywright() {
  try {
    return await import("playwright");
  } catch {
    throw new Error(
      "Playwright is required for explore_target. Install with: cd mcp && npm install && npx playwright install chromium",
    );
  }
}

export async function exploreTarget(
  client: ShadowSchemaClient,
  options: ExploreTargetOptions,
): Promise<ExploreTargetResult> {
  const startUrl = new URL(options.startUrl);
  if (shouldSkipUrl(startUrl)) {
    throw new Error(`start_url is not a crawlable HTTP(S) page: ${options.startUrl}`);
  }

  const maxPages = options.maxPages ?? 10;
  const maxDepth = options.maxDepth ?? 2;
  const waitMs = options.waitMs ?? 1_500;

  const before = await client.listEndpointsIndex(undefined, options.sessionId);
  const { chromium } = await loadPlaywright();

  const pagesVisited: string[] = [];
  const domainsHit = new Set<string>([startUrl.hostname]);
  const errors: string[] = [];
  const queued: Array<{ url: string; depth: number }> = [
    { url: normalizeUrl(startUrl), depth: 0 },
  ];
  const seen = new Set<string>();

  const browser = await chromium.launch({ headless: true });
  try {
    const context = await browser.newContext({
      proxy: { server: options.proxyUrl },
      ignoreHTTPSErrors: options.ignoreHTTPSErrors ?? true,
      storageState: options.storageStatePath,
    });
    const page = await context.newPage();

    while (queued.length > 0 && pagesVisited.length < maxPages) {
      const current = queued.shift();
      if (!current || seen.has(current.url)) {
        continue;
      }
      seen.add(current.url);

      try {
        await page.goto(current.url, { waitUntil: "domcontentloaded", timeout: 30_000 });
        await sleep(waitMs);

        pagesVisited.push(current.url);
        domainsHit.add(new URL(current.url).hostname);

        if (current.depth >= maxDepth) {
          continue;
        }

        const hrefs = await page.$$eval("a[href]", (anchors) =>
          anchors
            .map((anchor) => anchor.getAttribute("href"))
            .filter((href): href is string => Boolean(href)),
        );

        const links = extractSameOriginLinks(new URL(current.url), hrefs);
        for (const link of links) {
          if (!seen.has(link)) {
            queued.push({ url: link, depth: current.depth + 1 });
          }
        }
      } catch (error) {
        errors.push(`${current.url}: ${String(error)}`);
      }
    }

    await context.close();
  } finally {
    await browser.close();
  }

  const after = await client.listEndpointsIndex(undefined, options.sessionId);

  return {
    start_url: normalizeUrl(startUrl),
    pages_visited: pagesVisited,
    domains_hit: [...domainsHit].sort(),
    endpoint_count_before: before.count,
    endpoint_count_after: after.count,
    endpoint_delta: after.count - before.count,
    max_pages: maxPages,
    max_depth: maxDepth,
    errors,
  };
}