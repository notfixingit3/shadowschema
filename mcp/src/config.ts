export interface Config {
  exportUrl: string;
  proxyUrl: string;
  autoUpdate: boolean;
}

export function loadConfig(): Config {
  return {
    exportUrl: (process.env.SHADOWSCHEMA_EXPORT_URL ?? "http://localhost:38081").replace(
      /\/$/,
      "",
    ),
    proxyUrl: process.env.SHADOWSCHEMA_PROXY_URL ?? "http://127.0.0.1:38080",
    autoUpdate: process.env.SHADOWSCHEMA_AUTO_UPDATE !== "false",
  };
}