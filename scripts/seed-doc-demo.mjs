#!/usr/bin/env node
/**
 * Seeds synthetic traffic for README screenshots.
 * Requires the docs stack: docker compose -f docker-compose.yml -f docker-compose.docs.yml up -d
 */
import { createServer } from 'node:http';
import { WebSocketServer } from 'ws';

const API = process.env.SHADOWSCHEMA_API ?? 'http://127.0.0.1:38083';
const PROXY = process.env.SHADOWSCHEMA_PROXY ?? 'http://127.0.0.1:38082';
const MOCK_PORT = Number(process.env.MOCK_PORT ?? 9876);
// host.docker.internal lets the proxy container reach a mock API on the host (OrbStack / Docker Desktop).
const TARGET = process.env.DEMO_TARGET ?? 'host.docker.internal';

const routes = {
  'GET /api/v1/users': { users: [{ id: 1, name: 'Alex Chen', role: 'admin' }, { id: 2, name: 'Sam Rivera', role: 'viewer' }] },
  'GET /api/v1/users/42': { id: 42, name: 'Jordan Lee', email: 'jordan@acme-shop.demo', active: true },
  'GET /api/v1/products': { products: [{ sku: 'SKU-100', name: 'Field Kit', price: 49.99 }, { sku: 'SKU-220', name: 'Sensor Pack', price: 129.0 }] },
  'POST /api/v1/orders': { order_id: 'ord_8f2a91', status: 'created', total: 179.99 },
  'GET /api/v1/health': { status: 'ok', service: 'acme-shop-api', version: '2.4.1' },
};

function startMockServer() {
  const server = createServer((req, res) => {
    const key = `${req.method} ${req.url?.split('?')[0]}`;
    const body = routes[key];
    res.setHeader('Content-Type', 'application/json');
    res.setHeader('Access-Control-Allow-Origin', '*');
    if (!body) {
      res.statusCode = 404;
      res.end(JSON.stringify({ error: 'not_found', path: req.url }));
      return;
    }
    res.end(JSON.stringify(body));
  });

  const wss = new WebSocketServer({ server, path: '/ws/live' });
  wss.on('connection', (socket) => {
    socket.send(JSON.stringify({ event: 'welcome', channel: 'alerts' }));
    socket.on('message', (data) => {
      try {
        const msg = JSON.parse(String(data));
        if (msg.event === 'subscribe') {
          socket.send(JSON.stringify({ event: 'snapshot', items: [{ id: 'alert-1', level: 'info' }] }));
        }
      } catch {
        // ignore malformed frames in demo seed
      }
    });
  });

  server.listen(MOCK_PORT, '0.0.0.0', () => {
    console.log(`Mock API listening on http://0.0.0.0:${MOCK_PORT} (target host: ${TARGET})`);
  });

  return server;
}

async function api(path, { method = 'GET', body } = {}) {
  const res = await fetch(`${API}${path}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    throw new Error(`${method} ${path} -> ${res.status} ${await res.text()}`);
  }
  return res.json().catch(() => ({}));
}

async function proxyCurl(path, { method = 'GET', data } = {}) {
  const { execFile } = await import('node:child_process');
  const { promisify } = await import('node:util');
  const execFileAsync = promisify(execFile);
  const args = [
    '-sS', '-x', PROXY,
    '-X', method,
    '-H', 'Authorization: Bearer ss_demo_vault_token_7f3c',
    '-H', 'X-Api-Key: acme_demo_key_91bd',
    `http://${TARGET}:${MOCK_PORT}${path}`,
  ];
  if (data) {
    args.push('-H', 'Content-Type: application/json', '-d', data);
  }
  await execFileAsync('curl', args);
}

async function main() {
  startMockServer();
  await new Promise((r) => setTimeout(r, 300));

  let sessions = await api('/sessions');
  let acme = sessions.find((s) => s.target === TARGET);
  if (!acme) {
    await api('/sessions', {
      method: 'POST',
      body: {
        name: 'Acme Shop API',
        target: TARGET,
        ignore_rules: String.raw`\.(png|jpg|jpeg|webp|gif|css|js|woff|woff2|ico)$`,
      },
    });
    sessions = await api('/sessions');
    acme = sessions.find((s) => s.target === TARGET);
  }
  if (!acme) throw new Error('failed to create demo session');

  await api('/sessions/switch', { method: 'POST', body: { id: acme.id } });

  for (const path of ['/api/v1/health', '/api/v1/users', '/api/v1/users/42', '/api/v1/products']) {
    await proxyCurl(path);
  }

  await proxyCurl('/api/v1/orders', { method: 'POST', data: '{"sku":"SKU-220","qty":1}' });

  // Shadow domain: HTTPS CONNECT to an out-of-scope host
  const { execFile } = await import('node:child_process');
  const { promisify } = await import('node:util');
  const execFileAsync = promisify(execFile);
  await execFileAsync('curl', [
    '-sS', '-o', '/dev/null', '-x', PROXY, '-k', '--max-time', '5',
    'https://httpbin.org/get',
  ]).catch(() => {});

  // WebSocket upgrade through proxy (captured as endpoint metadata)
  await execFileAsync('curl', [
    '-sS', '-o', '/dev/null', '-x', PROXY,
    '-H', 'Connection: Upgrade',
    '-H', 'Upgrade: websocket',
    '-H', 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==',
    '-H', 'Sec-WebSocket-Version: 13',
    `http://${TARGET}:${MOCK_PORT}/ws/live`,
  ]).catch(() => {});

  // Allow debounced persistence to flush
  await new Promise((r) => setTimeout(r, 3000));

  const map = await api('/export-map');
  const paths = Object.keys(map.paths ?? {});
  console.log(`Seeded ${paths.length} paths: ${paths.join(', ')}`);
  console.log('Ready for screenshots at http://localhost:8082');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});