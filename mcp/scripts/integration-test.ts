#!/usr/bin/env tsx

import { loadConfig } from "../src/config.js";
import { ShadowSchemaClient } from "../src/client.js";

const config = loadConfig();
const client = new ShadowSchemaClient(config);

const checks: Array<{ name: string; run: () => Promise<void> }> = [
  {
    name: "health",
    run: async () => {
      const health = await client.getHealth();
      console.log(
        `  session: ${health.session_name} (${health.session_id}), endpoints: ${health.endpoint_count}`,
      );
    },
  },
  {
    name: "endpoints_index",
    run: async () => {
      const index = await client.listEndpointsIndex();
      console.log(`  indexed endpoints: ${index.count}`);
    },
  },
  {
    name: "list_sessions",
    run: async () => {
      const sessions = await client.listSessions();
      console.log(`  sessions: ${sessions.length}`);
    },
  },
  {
    name: "list_discovered_domains",
    run: async () => {
      const domains = await client.listDiscoveredDomains();
      console.log(`  discovered domains: ${domains.length}`);
    },
  },
  {
    name: "get_vault",
    run: async () => {
      const vault = await client.getVault();
      console.log(`  vault credentials: ${vault.length}`);
    },
  },
  {
    name: "get_ca_cert",
    run: async () => {
      const cert = await client.getCaCert();
      if (!cert.includes("BEGIN CERTIFICATE")) {
        throw new Error("CA cert does not look like PEM");
      }
      console.log(`  ca cert bytes: ${cert.length}`);
    },
  },
];

async function main() {
  console.log(`ShadowSchema integration test → ${config.exportUrl}`);

  let failed = 0;
  for (const check of checks) {
    process.stdout.write(`- ${check.name} ... `);
    try {
      await check.run();
      console.log("ok");
    } catch (error) {
      failed += 1;
      console.log("failed");
      console.error(`  ${String(error)}`);
    }
  }

  if (failed > 0) {
    console.error(`\n${failed} check(s) failed. Is ShadowSchema running? docker compose up -d`);
    process.exit(1);
  }

  console.log("\nAll integration checks passed.");
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});