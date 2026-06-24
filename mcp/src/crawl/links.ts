const STATIC_ASSET = /\.(png|jpe?g|gif|webp|svg|ico|css|js|mjs|map|woff2?|ttf|eot|pdf|zip)$/i;

export function normalizeUrl(url: URL): string {
  const copy = new URL(url.href);
  copy.hash = "";
  return copy.href;
}

export function shouldSkipUrl(url: URL): boolean {
  if (!["http:", "https:"].includes(url.protocol)) {
    return true;
  }
  if (STATIC_ASSET.test(url.pathname)) {
    return true;
  }
  return false;
}

export function isSameOrigin(base: URL, candidate: URL): boolean {
  return base.origin === candidate.origin;
}

export function extractSameOriginLinks(pageUrl: URL, hrefs: string[]): string[] {
  const links = new Set<string>();

  for (const href of hrefs) {
    if (!href || href.startsWith("mailto:") || href.startsWith("javascript:")) {
      continue;
    }

    try {
      const resolved = new URL(href, pageUrl.href);
      if (!isSameOrigin(pageUrl, resolved) || shouldSkipUrl(resolved)) {
        continue;
      }
      links.add(normalizeUrl(resolved));
    } catch {
      // ignore invalid URLs
    }
  }

  return [...links].sort();
}