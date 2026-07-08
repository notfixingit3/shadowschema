import type { ShadowSchemaClient } from "../client.js";
import { extractSameOriginLinks, normalizeUrl, shouldSkipUrl } from "./links.js";
import { sleep } from "../utils/sleep.js";

export type ExploreWaitUntil = "domcontentloaded" | "load" | "networkidle";

export interface ExploreTargetOptions {
  startUrl: string;
  maxPages?: number;
  maxDepth?: number;
  waitMs?: number;
  proxyUrl: string;
  storageStatePath?: string;
  ignoreHTTPSErrors?: boolean;
  sessionId?: number;
  /** Additional absolute URLs to seed the crawl queue (SPA routes, deep links). */
  seedUrls?: string[];
  /** CSS selectors to click on each page (nav items, tabs, menu buttons). */
  clickSelectors?: string[];
  /** Playwright waitUntil strategy (default networkidle for SPA-friendly capture). */
  waitUntil?: ExploreWaitUntil;
  /** When false, follow links to any http(s) host (default true = same origin only). */
  sameOriginOnly?: boolean;
  /** Extra hostnames allowed when sameOriginOnly is true (e.g. app CDN). */
  allowHosts?: string[];
}

export interface ExploreTargetResult {
  start_url: string;
  pages_visited: string[];
  domains_hit: string[];
  clicks_performed: number;
  endpoint_count_before: number;
  endpoint_count_after: number;
  endpoint_delta: number;
  max_pages: number;
  max_depth: number;
  wait_until: ExploreWaitUntil;
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

function hostAllowed(
  hostname: string,
  originHost: string,
  sameOriginOnly: boolean,
  allowHosts: string[],
): boolean {
  if (!sameOriginOnly) {
    return true;
  }
  if (hostname === originHost) {
    return true;
  }
  return allowHosts.some((h) => h === hostname || hostname.endsWith(`.${h}`));
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
  const waitUntil: ExploreWaitUntil = options.waitUntil ?? "networkidle";
  const sameOriginOnly = options.sameOriginOnly ?? true;
  const allowHosts = (options.allowHosts ?? []).map((h) => h.toLowerCase());
  const clickSelectors = options.clickSelectors ?? [
    "nav a[href]",
    "header a[href]",
    "[role='navigation'] a[href]",
    "a[href].nav-link",
    "button[data-testid*='nav']",
  ];

  const before = await client.listEndpointsIndex(undefined, options.sessionId);
  const { chromium } = await loadPlaywright();

  const pagesVisited: string[] = [];
  const domainsHit = new Set<string>([startUrl.hostname]);
  const errors: string[] = [];
  let clicksPerformed = 0;

  const queued: Array<{ url: string; depth: number }> = [
    { url: normalizeUrl(startUrl), depth: 0 },
  ];
  for (const seed of options.seedUrls ?? []) {
    try {
      const u = new URL(seed, startUrl);
      if (!shouldSkipUrl(u)) {
        queued.push({ url: normalizeUrl(u), depth: 0 });
      }
    } catch {
      errors.push(`invalid seed_url: ${seed}`);
    }
  }

  const seen = new Set<string>();

  const browser = await chromium.launch({
    headless: true,
    args: ["--ignore-certificate-errors"],
  });
  try {
    const context = await browser.newContext({
      proxy: { server: options.proxyUrl },
      ignoreHTTPSErrors: options.ignoreHTTPSErrors ?? true,
      storageState: options.storageStatePath,
    });
    const page = await context.newPage();

    // Collect SPA client-side navigations as crawl candidates.
    page.on("framenavigated", (frame) => {
      if (frame !== page.mainFrame()) {
        return;
      }
      try {
        const u = new URL(frame.url());
        if (
          !shouldSkipUrl(u) &&
          hostAllowed(u.hostname, startUrl.hostname, sameOriginOnly, allowHosts)
        ) {
          const normalized = normalizeUrl(u);
          if (!seen.has(normalized) && !queued.some((q) => q.url === normalized)) {
            queued.push({ url: normalized, depth: 0 });
          }
        }
      } catch {
        // ignore
      }
    });

    while (queued.length > 0 && pagesVisited.length < maxPages) {
      const current = queued.shift();
      if (!current || seen.has(current.url)) {
        continue;
      }
      seen.add(current.url);

      try {
        await page.goto(current.url, { waitUntil, timeout: 45_000 });
        // Extra settle time for late XHR after networkidle.
        await sleep(waitMs);

        pagesVisited.push(current.url);
        domainsHit.add(new URL(current.url).hostname);

        // Click interactive selectors to surface more API traffic (tabs, menus).
        for (const selector of clickSelectors) {
          if (pagesVisited.length >= maxPages) {
            break;
          }
          const locators = page.locator(selector);
          const count = Math.min(await locators.count(), 8);
          for (let i = 0; i < count; i++) {
            try {
              const el = locators.nth(i);
              if (!(await el.isVisible())) {
                continue;
              }
              await el.click({ timeout: 3_000, trial: false });
              clicksPerformed++;
              try {
                await page.waitForLoadState(waitUntil === "domcontentloaded" ? "domcontentloaded" : "networkidle", {
                  timeout: 10_000,
                });
              } catch {
                // non-fatal
              }
              await sleep(Math.min(waitMs, 800));
              const afterClick = normalizeUrl(new URL(page.url()));
              if (
                !seen.has(afterClick) &&
                hostAllowed(
                  new URL(afterClick).hostname,
                  startUrl.hostname,
                  sameOriginOnly,
                  allowHosts,
                )
              ) {
                queued.push({ url: afterClick, depth: current.depth });
              }
            } catch {
              // selector may not be clickable; continue
            }
          }
        }

        if (current.depth >= maxDepth) {
          continue;
        }

        const hrefs = await page.$$eval("a[href]", (anchors) =>
          anchors
            .map((anchor) => anchor.getAttribute("href"))
            .filter((href): href is string => Boolean(href)),
        );

        const pageUrl = new URL(current.url);
        let links: string[];
        if (sameOriginOnly && allowHosts.length === 0) {
          links = extractSameOriginLinks(pageUrl, hrefs);
        } else {
          links = [];
          for (const href of hrefs) {
            try {
              const absolute = new URL(href, pageUrl);
              if (shouldSkipUrl(absolute)) {
                continue;
              }
              if (
                !hostAllowed(
                  absolute.hostname,
                  startUrl.hostname,
                  sameOriginOnly,
                  allowHosts,
                )
              ) {
                continue;
              }
              links.push(normalizeUrl(absolute));
            } catch {
              // skip bad href
            }
          }
        }

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
    clicks_performed: clicksPerformed,
    endpoint_count_before: before.count,
    endpoint_count_after: after.count,
    endpoint_delta: after.count - before.count,
    max_pages: maxPages,
    max_depth: maxDepth,
    wait_until: waitUntil,
    errors,
  };
}
