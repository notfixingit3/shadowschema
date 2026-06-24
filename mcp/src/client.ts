import type { Config } from "./config.js";

export interface SessionMeta {
  id: number;
  name: string;
  target: string;
  ignore_rules: string;
  updated_at: string;
}

export interface AuthCredential {
  header_name: string;
  token_value: string;
  first_seen: string;
}

export interface HealthResponse {
  status: string;
  session_id: number;
  session_name: string;
  target: string;
  endpoint_count: number;
  active_session: boolean;
  session_updated_at: string;
}

export interface EndpointIndexEntry {
  path: string;
  methods: string[];
  last_seen?: string;
  has_payload: boolean;
  websocket: boolean;
}

export interface EndpointIndexResponse {
  count: number;
  session_id: number;
  endpoints: EndpointIndexEntry[];
  path_prefix?: string;
}

export type OpenApiDocument = Record<string, unknown>;

export class ShadowSchemaClient {
  constructor(private readonly config: Config) {}

  get exportUrl(): string {
    return this.config.exportUrl;
  }

  get proxyUrl(): string {
    return this.config.proxyUrl;
  }

  async getHealth(sessionId?: number): Promise<HealthResponse> {
    const response = await this.request(this.withQuery("/health", { session_id: sessionId }));
    return response.json() as Promise<HealthResponse>;
  }

  async listEndpointsIndex(
    pathPrefix?: string,
    sessionId?: number,
  ): Promise<EndpointIndexResponse> {
    const response = await this.request(
      this.withQuery("/endpoints", {
        path_prefix: pathPrefix,
        session_id: sessionId,
      }),
    );
    return response.json() as Promise<EndpointIndexResponse>;
  }

  async getEndpointFromApi(path: string, sessionId?: number): Promise<Record<string, unknown>> {
    const trimmed = path.startsWith("/") ? path.slice(1) : path;
    const response = await this.request(
      this.withQuery(`/endpoints/${trimmed}`, { session_id: sessionId }),
    );
    return response.json() as Promise<Record<string, unknown>>;
  }

  async getSpec(
    format: "json" | "yaml" = "json",
    options: { pathPrefix?: string; sessionId?: number } = {},
  ): Promise<string> {
    const response = await this.request(
      this.withQuery("/export-map", {
        format: format === "yaml" ? "yaml" : undefined,
        path_prefix: options.pathPrefix,
        session_id: options.sessionId,
      }),
    );
    return response.text();
  }

  async getSpecJson(options: {
    pathPrefix?: string;
    sessionId?: number;
  } = {}): Promise<OpenApiDocument> {
    const text = await this.getSpec("json", options);
    return JSON.parse(text) as OpenApiDocument;
  }

  async listSessions(): Promise<SessionMeta[]> {
    const response = await this.request("/sessions");
    return response.json() as Promise<SessionMeta[]>;
  }

  async createSession(name: string, target: string, ignoreRules = ""): Promise<void> {
    await this.post("/sessions", { name, target, ignore_rules: ignoreRules });
  }

  async switchSession(id: number): Promise<void> {
    await this.post("/sessions/switch", { id });
  }

  async listDiscoveredDomains(): Promise<string[]> {
    const response = await this.request("/discovered");
    return response.json() as Promise<string[]>;
  }

  async getVault(): Promise<AuthCredential[]> {
    const response = await this.request("/vault");
    return response.json() as Promise<AuthCredential[]>;
  }

  async addTargetDomain(domain: string): Promise<void> {
    await this.post("/sessions/add-target", { domain });
  }

  async generateSdk(language: string): Promise<{ data: Buffer; excluded: number | null }> {
    const response = await fetch(`${this.config.exportUrl}/generate-sdk`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ language }),
    });

    if (!response.ok) {
      const body = await response.text();
      throw new Error(`POST /generate-sdk failed (${response.status}): ${body}`);
    }

    const excludedHeader = response.headers.get("X-ShadowSchema-WebSocket-Excluded");
    const excluded = excludedHeader ? Number.parseInt(excludedHeader, 10) : null;
    const data = Buffer.from(await response.arrayBuffer());
    return { data, excluded };
  }

  async getCaCert(): Promise<string> {
    const response = await this.request("/ca-cert");
    return response.text();
  }

  async exportReplay(path: string, method: string, sessionId?: number): Promise<string> {
    const response = await this.request(
      this.withQuery("/export-replay", {
        path,
        method,
        session_id: sessionId,
      }),
    );
    return response.text();
  }

  async healthCheck(): Promise<{
    ok: boolean;
    endpointCount: number;
    title: string;
    sessionId?: number;
    sessionName?: string;
    target?: string;
  }> {
    try {
      const health = await this.getHealth();
      return {
        ok: health.status === "ok",
        endpointCount: health.endpoint_count,
        title: health.session_name || "ShadowSchema Auto-Generated API",
        sessionId: health.session_id,
        sessionName: health.session_name,
        target: health.target,
      };
    } catch {
      const spec = await this.getSpecJson();
      const paths = (spec.paths as Record<string, unknown> | undefined) ?? {};
      const info = (spec.info as Record<string, unknown> | undefined) ?? {};
      return {
        ok: true,
        endpointCount: Object.keys(paths).length,
        title: String(info.title ?? "ShadowSchema Auto-Generated API"),
      };
    }
  }

  private withQuery(
    path: string,
    params: Record<string, string | number | undefined>,
  ): string {
    const search = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== "") {
        search.set(key, String(value));
      }
    }
    const query = search.toString();
    return query ? `${path}?${query}` : path;
  }

  private async request(path: string): Promise<Response> {
    const response = await fetch(`${this.config.exportUrl}${path}`);
    if (!response.ok) {
      const body = await response.text();
      throw new Error(`GET ${path} failed (${response.status}): ${body}`);
    }
    return response;
  }

  private async post(path: string, body: unknown): Promise<void> {
    const response = await fetch(`${this.config.exportUrl}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`POST ${path} failed (${response.status}): ${text}`);
    }
  }
}