#!/usr/bin/env node
import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const OUT_DIR = path.resolve(__dirname, '../docs/screenshots');
const BASE_URL = process.env.DASHBOARD_URL ?? 'http://127.0.0.1:8082';

async function shot(page, name) {
  const file = path.join(OUT_DIR, name);
  await page.screenshot({ path: file, fullPage: false });
  console.log(`Wrote ${file}`);
}

async function main() {
  await mkdir(OUT_DIR, { recursive: true });

  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await page.waitForSelector('#connection-status', { timeout: 15000 });
  await page.waitForFunction(() => {
    const list = document.getElementById('endpoint-list');
    return list && list.querySelectorAll('li').length >= 3;
  }, { timeout: 20000 });

  await shot(page, 'dashboard-overview.png');

  const firstEndpoint = page.locator('#endpoint-list li').first();
  await firstEndpoint.click();
  await page.waitForSelector('#endpoint-details:not(.hidden)', { timeout: 5000 });
  await page.waitForTimeout(500);
  await shot(page, 'endpoint-detail.png');

  await page.locator('#new-session-btn').click();
  await page.waitForSelector('#new-session-modal:not(.hidden)', { timeout: 5000 });
  await page.waitForTimeout(300);
  await shot(page, 'new-session-modal.png');
  await page.locator('#ns-cancel').click();

  await page.locator('#vault-btn').click();
  await page.waitForSelector('#vault-modal:not(.hidden)', { timeout: 5000 });
  await page.waitForTimeout(500);
  await shot(page, 'vault-modal.png');
  await page.locator('#vault-close').click();

  await page.locator('#view-domains-btn').click();
  await page.waitForSelector('#discovered-domains-modal:not(.hidden)', { timeout: 5000 });
  await page.waitForTimeout(500);
  await shot(page, 'shadow-domains.png');
  await page.locator('#dd-close').click();

  await browser.close();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});