import type { OpenApiDocument } from "../client.js";

const HTTP_METHODS = new Set([
  "get",
  "put",
  "post",
  "delete",
  "options",
  "head",
  "patch",
  "trace",
]);

export interface EndpointSummary {
  path: string;
  methods: string[];
  websocket: boolean;
  hasLastPayload: boolean;
  hasWebsocketSchemas: boolean;
}

function operationHasExtension(operation: Record<string, unknown>, key: string): boolean {
  return operation[key] !== undefined;
}

function collectMethods(pathItem: Record<string, unknown>): string[] {
  return Object.keys(pathItem).filter((key) => HTTP_METHODS.has(key));
}

export function listEndpointSummaries(
  spec: OpenApiDocument,
  pathPrefix?: string,
): EndpointSummary[] {
  const paths = (spec.paths as Record<string, Record<string, unknown>> | undefined) ?? {};
  const summaries: EndpointSummary[] = [];

  for (const [path, pathItem] of Object.entries(paths)) {
    if (pathPrefix && !path.startsWith(pathPrefix)) {
      continue;
    }

    const methods = collectMethods(pathItem);
    let websocket = false;
    let hasLastPayload = false;
    let hasWebsocketSchemas = false;

    for (const method of methods) {
      const operation = pathItem[method] as Record<string, unknown> | undefined;
      if (!operation) {
        continue;
      }
      if (operationHasExtension(operation, "x-websocket")) {
        websocket = true;
      }
      if (operationHasExtension(operation, "x-last-payload")) {
        hasLastPayload = true;
      }
      if (
        operationHasExtension(operation, "x-websocket-message-schema-in") ||
        operationHasExtension(operation, "x-websocket-message-schema-out")
      ) {
        hasWebsocketSchemas = true;
      }
    }

    summaries.push({
      path,
      methods: methods.map((m) => m.toUpperCase()),
      websocket,
      hasLastPayload,
      hasWebsocketSchemas,
    });
  }

  summaries.sort((a, b) => a.path.localeCompare(b.path));
  return summaries;
}

export function filterSpecByPathPrefix(
  spec: OpenApiDocument,
  pathPrefix?: string,
): OpenApiDocument {
  if (!pathPrefix) {
    return spec;
  }

  const paths = (spec.paths as Record<string, unknown> | undefined) ?? {};
  const filteredPaths: Record<string, unknown> = {};

  for (const [path, pathItem] of Object.entries(paths)) {
    if (path.startsWith(pathPrefix)) {
      filteredPaths[path] = pathItem;
    }
  }

  return {
    ...spec,
    paths: filteredPaths,
  };
}

export function getEndpointDetail(
  spec: OpenApiDocument,
  path: string,
): Record<string, unknown> | null {
  const paths = (spec.paths as Record<string, Record<string, unknown>> | undefined) ?? {};
  const pathItem = paths[path];
  if (!pathItem) {
    return null;
  }

  const methods = collectMethods(pathItem);
  const operations: Record<string, unknown> = {};

  for (const method of methods) {
    operations[method] = pathItem[method];
  }

  return {
    path,
    methods: methods.map((m) => m.toUpperCase()),
    operations,
  };
}