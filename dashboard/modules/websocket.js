import { syntaxHighlight } from './highlight.js';

export function isWebSocketOperation(method, operation) {
  if (operation && operation['x-websocket'] === true) return true;
  return method.toLowerCase() === 'trace';
}

export function displayMethodFor(method, operation) {
  return isWebSocketOperation(method, operation) ? 'WS' : method.toUpperCase();
}

export function renderWebSocketStats(stats) {
  if (!stats) {
    return '<div class="ws-stats-empty">No frames intercepted yet.</div>';
  }

  const items = [
    ['Total', stats.total || 0],
    ['Data', stats.data || 0],
    ['Control', stats.control || 0],
    ['Inbound', stats.in || 0],
    ['Outbound', stats.out || 0],
    ['Fragmented', stats.fragmented || 0],
  ];

  return `
    <div class="ws-stats-grid">
      ${items
        .map(
          ([label, value]) => `
        <div class="ws-stat-item">
          <span class="ws-stat-label">${label}</span>
          <span class="ws-stat-value">${value}</span>
        </div>
      `,
        )
        .join('')}
    </div>
  `;
}

export function formatWebSocketPayload(payload) {
  if (payload == null) return '<span class="ws-payload-empty">(empty)</span>';
  if (typeof payload === 'string') return `<code>${payload}</code>`;
  if (payload.close_code !== undefined) {
    const reason = payload.close_reason ? ` — ${payload.close_reason}` : '';
    return `<code>close ${payload.close_code}${reason}</code>`;
  }
  if (payload.encoding === 'base64') {
    return `<code>[binary ${payload.size || 0} bytes]</code>`;
  }
  return `<code>${JSON.stringify(payload)}</code>`;
}

export function renderWebSocketSchemas(operation) {
  const inbound = operation['x-websocket-message-schema-in'];
  const outbound = operation['x-websocket-message-schema-out'];
  const legacy = operation['x-websocket-message-schema'];

  const blocks = [];

  if (outbound) {
    blocks.push(`
      <div style="margin-top: 1.25rem;">
        <div style="color: #60a5fa; font-size: 0.85rem; margin-bottom: 0.5rem;">Outbound Messages (client → server)</div>
        ${syntaxHighlight(outbound)}
      </div>
    `);
  }

  if (inbound) {
    blocks.push(`
      <div style="margin-top: 1.25rem;">
        <div style="color: #34d399; font-size: 0.85rem; margin-bottom: 0.5rem;">Inbound Messages (server → client)</div>
        ${syntaxHighlight(inbound)}
      </div>
    `);
  }

  if (blocks.length === 0 && legacy) {
    blocks.push(`
      <div style="margin-top: 1.25rem;">
        <div style="color: #38bdf8; font-size: 0.85rem; margin-bottom: 0.5rem;">Inferred Message Schema</div>
        ${syntaxHighlight(legacy)}
      </div>
    `);
  }

  if (blocks.length === 0) {
    return `<div style="color: #64748b; margin-top: 1rem; font-size: 0.9rem;">Directional message schemas will appear here as client and server frames are intercepted.</div>`;
  }

  return blocks.join('');
}

export function renderWebSocketFrameLog(frames) {
  if (!frames || frames.length === 0) {
    return '<div class="ws-frame-empty">Waiting for intercepted WebSocket frames...</div>';
  }

  const rows = [...frames]
    .reverse()
    .map((frame) => {
      const direction = frame.direction === 'in' ? 'IN' : 'OUT';
      const directionClass = frame.direction === 'in' ? 'ws-dir-in' : 'ws-dir-out';
      const frag = frame.fragmented
        ? `<span class="ws-frag-badge">${frame.fragments} frags</span>`
        : '';
      const time = frame.captured_at ? new Date(frame.captured_at).toLocaleTimeString() : '';
      return `
      <div class="ws-frame-row">
        <div class="ws-frame-meta">
          <span class="ws-dir-badge ${directionClass}">${direction}</span>
          <span class="ws-opcode-badge">${(frame.opcode_name || 'unknown').toUpperCase()}</span>
          ${frag}
          <span class="ws-frame-time">${time}</span>
        </div>
        <div class="ws-frame-payload">${formatWebSocketPayload(frame.payload)}</div>
      </div>
    `;
    })
    .join('');

  return `<div class="ws-frame-log">${rows}</div>`;
}
