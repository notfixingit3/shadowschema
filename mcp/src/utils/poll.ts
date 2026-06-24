import type { ShadowSchemaClient } from "../client.js";
import { sleep } from "./sleep.js";

export interface WaitForEndpointsOptions {
  minCount?: number;
  pathPrefix?: string;
  timeoutMs?: number;
  intervalMs?: number;
  sessionId?: number;
}

export interface WaitForEndpointsResult {
  satisfied: boolean;
  endpointCount: number;
  matchingCount: number;
  waitedMs: number;
  endpoints: Awaited<ReturnType<ShadowSchemaClient["listEndpointsIndex"]>>["endpoints"];
}

export async function waitForEndpoints(
  client: ShadowSchemaClient,
  options: WaitForEndpointsOptions = {},
): Promise<WaitForEndpointsResult> {
  const minCount = options.minCount ?? 1;
  const timeoutMs = options.timeoutMs ?? 60_000;
  const intervalMs = options.intervalMs ?? 2_000;
  const started = Date.now();

  while (Date.now() - started < timeoutMs) {
    const index = await client.listEndpointsIndex(options.pathPrefix, options.sessionId);
    const matchingCount = index.endpoints.length;
    const totalCount = index.count;

    if (matchingCount >= minCount) {
      return {
        satisfied: true,
        endpointCount: totalCount,
        matchingCount,
        waitedMs: Date.now() - started,
        endpoints: index.endpoints,
      };
    }

    await sleep(intervalMs);
  }

  const finalIndex = await client.listEndpointsIndex(options.pathPrefix, options.sessionId);
  return {
    satisfied: false,
    endpointCount: finalIndex.count,
    matchingCount: finalIndex.endpoints.length,
    waitedMs: Date.now() - started,
    endpoints: finalIndex.endpoints,
  };
}