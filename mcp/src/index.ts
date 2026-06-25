#!/usr/bin/env node

import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { ShadowSchemaClient } from "./client.js";
import { loadConfig } from "./config.js";
import { createServer } from "./server.js";
import { tryAutoUpdate } from "./utils/update.js";
import path from "path";
import { fileURLToPath } from "url";

async function main() {
  const config = loadConfig();
  const client = new ShadowSchemaClient(config);

  // Check for updates in the background
  const __filename = fileURLToPath(import.meta.url);
  const mcpDir = path.resolve(path.dirname(__filename), "..");
  tryAutoUpdate(mcpDir, config.autoUpdate).catch((error) => {
    console.error("[shadowschema-mcp] Auto-update warning:", error);
  });

  const server = createServer(client);
  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch((error) => {
  console.error("shadowschema-mcp failed:", error);
  process.exit(1);
});