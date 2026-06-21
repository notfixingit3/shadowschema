import './style.css';
import { registerSW } from 'virtual:pwa-register';

registerSW({ immediate: true });

const API_URL = import.meta.env.VITE_API_URL ?? '';

const proxyHint = document.getElementById('proxy-hint');
if (proxyHint) {
  const host = window.location.hostname === 'localhost' ? 'localhost' : window.location.hostname;
  proxyHint.textContent = `${host}:38080`;
}

const statusText = document.getElementById('connection-status');
const pulse = document.querySelector('.pulse');
const endpointList = document.getElementById('endpoint-list');
const welcomeState = document.getElementById('welcome-state');
const endpointDetails = document.getElementById('endpoint-details');
const detailPanel = document.getElementById('detail-panel');

const elMethod = document.getElementById('endpoint-method');
const elPath = document.getElementById('endpoint-path');
const elParams = document.getElementById('endpoint-params');
const elResponse = document.getElementById('endpoint-response');
const elRaw = document.getElementById('endpoint-raw');
const copyPythonBtn = document.getElementById('copy-python-btn');
const exportBtn = document.getElementById('export-json-btn');
const exportYamlBtn = document.getElementById('export-yaml-btn');
const sdkButtons = {
  python: document.getElementById('gen-sdk-python-btn'),
  'typescript-fetch': document.getElementById('gen-sdk-ts-btn'),
  go: document.getElementById('gen-sdk-go-btn'),
  rust: document.getElementById('gen-sdk-rust-btn'),
};

// Session elements
const sessionSelect = document.getElementById('session-select');
const newSessionBtn = document.getElementById('new-session-btn');
const modal = document.getElementById('new-session-modal');
const btnCancel = document.getElementById('ns-cancel');
const btnCreate = document.getElementById('ns-create');
const inputName = document.getElementById('ns-name');
const inputTarget = document.getElementById('ns-target');
const inputIgnore = document.getElementById('ns-ignore');

// Admin elements
const manageBtn = document.getElementById('manage-sessions-btn');
const adminModal = document.getElementById('manage-sessions-modal');
const adminClose = document.getElementById('ms-close');
const adminList = document.getElementById('session-admin-list');

// CA cert download
const downloadCABtn = document.getElementById('download-ca-btn');
if (downloadCABtn) {
  downloadCABtn.addEventListener('click', () => {
    const link = document.createElement('a');
    link.href = `${API_URL}/ca-cert`;
    link.download = 'shadowschema-ca.crt';
    link.rel = 'noopener';
    document.body.appendChild(link);
    link.click();
    link.remove();
  });
}

// Vault elements
const vaultBtn = document.getElementById('vault-btn');
const vaultModal = document.getElementById('vault-modal');
const vaultClose = document.getElementById('vault-close');
const vaultList = document.getElementById('vault-list');

// Discovered Domains
const viewDomainsBtn = document.getElementById('view-domains-btn');
const discoveredModal = document.getElementById('discovered-domains-modal');
const ddClose = document.getElementById('dd-close');
const discoveredList = document.getElementById('discovered-admin-list');

// Metric Elements
const statRoutes = document.getElementById('stat-routes');
const statEndpoints = document.getElementById('stat-endpoints');
const statDomains = document.getElementById('stat-domains');
const statTargets = document.getElementById('stat-targets');

// UI elements
const searchInput = document.getElementById('endpoint-search');
const methodFilters = document.getElementById('method-filters');
const tabSchema = document.getElementById('tab-schema');
const tabRaw = document.getElementById('tab-raw');


// Tab logic
tabSchema.addEventListener('click', () => {
  tabSchema.classList.add('active');
  tabRaw.classList.remove('active');
  elResponse.classList.remove('hidden');
  elRaw.classList.add('hidden');
});

tabRaw.addEventListener('click', () => {
  tabRaw.classList.add('active');
  tabSchema.classList.remove('active');
  elRaw.classList.remove('hidden');
  elResponse.classList.add('hidden');
});

// Search + method filter logic
let methodFilter = 'all';

if (searchInput) {
  searchInput.addEventListener('input', () => renderSidebar());
}

if (methodFilters) {
  methodFilters.addEventListener('click', (e) => {
    const btn = e.target.closest('.method-filter-btn');
    if (!btn) return;
    methodFilter = btn.dataset.filter || 'all';
    methodFilters.querySelectorAll('.method-filter-btn').forEach((el) => {
      el.classList.toggle('active', el === btn);
    });
    renderSidebar();
  });
}

// Vault logic
if (vaultBtn && vaultModal && vaultClose && vaultList) {
  vaultBtn.addEventListener('click', () => {
    vaultModal.classList.remove('hidden');
    vaultList.innerHTML = '<tr><td colspan="4" style="text-align:center; padding: 1rem;">Loading...</td></tr>';
    
    fetch(`${API_URL}/vault`)
      .then(res => res.json())
      .then(creds => {
        vaultList.innerHTML = '';
        if (!creds || creds.length === 0) {
          vaultList.innerHTML = '<tr><td colspan="4" style="text-align:center; padding: 1rem;">No credentials captured yet.</td></tr>';
          return;
        }
        creds.forEach(c => {
          const tr = document.createElement('tr');
          tr.style.borderBottom = '1px solid rgba(255,255,255,0.05)';

          const headerCell = document.createElement('td');
          headerCell.style.padding = '0.75rem 0.5rem';
          headerCell.style.fontFamily = 'var(--font-mono)';
          headerCell.style.color = 'var(--accent-cyan)';
          headerCell.textContent = c.header_name;

          const valueCell = document.createElement('td');
          valueCell.style.padding = '0.75rem 0.5rem';
          valueCell.style.fontFamily = 'var(--font-mono)';
          valueCell.style.wordBreak = 'break-all';
          valueCell.textContent = c.token_value;

          const seenCell = document.createElement('td');
          seenCell.style.padding = '0.75rem 0.5rem';
          seenCell.style.fontSize = '0.85rem';
          seenCell.style.color = 'var(--text-muted)';
          seenCell.textContent = new Date(c.first_seen).toLocaleString();

          const actionCell = document.createElement('td');
          actionCell.style.padding = '0.75rem 0.5rem';
          const copyBtn = document.createElement('button');
          copyBtn.className = 'glass-btn small';
          copyBtn.textContent = 'Copy';
          copyBtn.addEventListener('click', () => {
            navigator.clipboard.writeText(c.token_value).then(() => {
              const original = copyBtn.textContent;
              copyBtn.textContent = '✓';
              setTimeout(() => { copyBtn.textContent = original; }, 1500);
            });
          });
          actionCell.appendChild(copyBtn);

          tr.append(headerCell, valueCell, seenCell, actionCell);
          vaultList.appendChild(tr);
        });
      })
      .catch(err => {
        vaultList.innerHTML = `<tr><td colspan="4" style="text-align:center; padding: 1rem; color: red;">Error: ${err}</td></tr>`;
      });
  });

  vaultClose.addEventListener('click', () => {
    vaultModal.classList.add('hidden');
  });
}

let currentSpec = null;
let selectedPath = null;
let selectedMethod = null;
let currentSessionId = null;
let currentSessionName = null;

const HTTP_METHODS = new Set(['get', 'post', 'put', 'delete', 'patch', 'head', 'options', 'trace', 'connect']);

function isWebSocketOperation(method, operation) {
  if (operation && operation['x-websocket'] === true) return true;
  return method.toLowerCase() === 'trace';
}

function displayMethodFor(method, operation) {
  return isWebSocketOperation(method, operation) ? 'WS' : method.toUpperCase();
}

function matchesMethodFilter(method, operation) {
  if (methodFilter === 'all') return true;
  const normalized = method.toLowerCase();
  if (methodFilter === 'ws') return isWebSocketOperation(method, operation);
  if (methodFilter === 'get') return normalized === 'get';
  if (methodFilter === 'post') return ['post', 'put', 'patch', 'delete'].includes(normalized);
  return true;
}

function sessionExportSlug() {
  const raw = currentSessionName || `session_${currentSessionId || 'export'}`;
  const slug = raw.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_|_$/g, '');
  return slug || 'session';
}

function downloadBlob(blob, filename) {
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  window.URL.revokeObjectURL(url);
}

function renderWebSocketStats(stats) {
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
      ${items.map(([label, value]) => `
        <div class="ws-stat-item">
          <span class="ws-stat-label">${label}</span>
          <span class="ws-stat-value">${value}</span>
        </div>
      `).join('')}
    </div>
  `;
}

function formatWebSocketPayload(payload) {
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

function renderWebSocketSchemas(operation) {
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

function vaultHeadersFromSpec(spec) {
  const headers = {};
  const vault = spec?.['x-shadowschema-vault'];
  if (!Array.isArray(vault)) return headers;

  vault.forEach(c => {
    if (c.header_name && c.token_value) {
      headers[c.header_name] = c.token_value;
    }
  });
  return headers;
}

async function resolveVaultHeaders(spec) {
  let headers = vaultHeadersFromSpec(spec);
  if (Object.keys(headers).length > 0) {
    return headers;
  }

  try {
    const res = await fetch(`${API_URL}/vault`);
    if (!res.ok) return headers;
    const creds = await res.json();
    creds.forEach(c => {
      if (c.header_name && c.token_value) {
        headers[c.header_name] = c.token_value;
      }
    });
  } catch (err) {
    console.error('Failed to fetch vault credentials', err);
  }
  return headers;
}

function renderWebSocketFrameLog(frames) {
  if (!frames || frames.length === 0) {
    return '<div class="ws-frame-empty">Waiting for intercepted WebSocket frames...</div>';
  }

  const rows = [...frames].reverse().map(frame => {
    const direction = frame.direction === 'in' ? 'IN' : 'OUT';
    const directionClass = frame.direction === 'in' ? 'ws-dir-in' : 'ws-dir-out';
    const frag = frame.fragmented ? `<span class="ws-frag-badge">${frame.fragments} frags</span>` : '';
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
  }).join('');

  return `<div class="ws-frame-log">${rows}</div>`;
}

async function fetchSessions() {
  try {
    const res = await fetch(`${API_URL}/sessions`);
    if (!res.ok) return;
    const sessions = await res.json();
    
    const currentOptions = Array.from(sessionSelect.options).map(o => o.value);
    const newOptions = sessions.map(s => s.id.toString());
    
    if (currentOptions.join() !== newOptions.join()) {
      sessionSelect.innerHTML = '';
      sessions.forEach(s => {
        const opt = document.createElement('option');
        opt.value = s.id;
        opt.textContent = `Target: ${s.target} — ${s.name}`;
        sessionSelect.appendChild(opt);
      });
      if (sessions.length > 0 && !currentSessionId) {
        currentSessionId = sessions[0].id.toString();
        sessionSelect.value = currentSessionId;
      } else if (currentSessionId) {
        sessionSelect.value = currentSessionId;
      }
    }
    
    const active = sessions.find(s => s.id.toString() === currentSessionId);
    if (active) {
      currentSessionName = active.name;
      statTargets.textContent = active.target.split(',').join(', ');
    }
  } catch (err) {
    console.error("Failed to fetch sessions", err);
  }
}

async function switchSession(id) {
  try {
    await fetch(`${API_URL}/sessions/switch`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({id: parseInt(id)})
    });
    currentSessionId = id;
    currentSpec = null;
    fetchSpec();
  } catch (err) {
    console.error(err);
  }
}

async function fetchSpec() {
  try {
    await fetchSessions();

    const res = await fetch(`${API_URL}/export-map`);
    if (!res.ok) throw new Error('Network response was not ok');
    const data = await res.json();
    
    statusText.textContent = 'Listening (Secure)';
    pulse.classList.remove('error');
    exportBtn.disabled = false;
    if(exportYamlBtn) exportYamlBtn.disabled = false;
    setSdkButtonsDisabled(false);
    
    if (JSON.stringify(data) !== JSON.stringify(currentSpec)) {
      currentSpec = data;
      renderSidebar();
      if (selectedPath && selectedMethod) {
        renderDetails(selectedPath, selectedMethod);
      }
    }
  } catch (err) {
    statusText.textContent = 'Proxy Offline';
    pulse.classList.add('error');
    exportBtn.disabled = true;
    if(exportYamlBtn) exportYamlBtn.disabled = true;
    setSdkButtonsDisabled(true);
  }
}

function renderSidebar() {
  endpointList.innerHTML = '';
  let count = 0;
  
  if (!currentSpec || !currentSpec.paths) {
    statEndpoints.textContent = "0";
    statRoutes.textContent = "0";
    return;
  }

  const searchQuery = searchInput?.value?.toLowerCase().trim() || '';
  const entries = [];

  Object.entries(currentSpec.paths).forEach(([path, methods]) => {
    if (searchQuery && !path.toLowerCase().includes(searchQuery)) return;

    Object.keys(methods).forEach(method => {
      if (!HTTP_METHODS.has(method.toLowerCase())) return;

      const operation = methods[method];
      if (!matchesMethodFilter(method, operation)) return;

      entries.push({ path, method, operation });
    });
  });

  entries.sort((a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method));

  const uniqueRoutes = new Set(entries.map((entry) => entry.path));
  statRoutes.textContent = uniqueRoutes.size;

  entries.forEach(({ path, method, operation }) => {
    count++;
    const li = document.createElement('li');
    li.className = 'endpoint-item';
    if (path === selectedPath && method === selectedMethod.toLowerCase()) {
      li.classList.add('active');
    }

    const displayMethod = displayMethodFor(method, operation);

    li.innerHTML = `
      <span class="method-badge badge-${displayMethod}">${displayMethod}</span>
      <span class="endpoint-path-label">${path}</span>
    `;

    li.onclick = () => {
      selectedPath = path;
      selectedMethod = method.toUpperCase();
      renderSidebar();
      renderDetails(path, method.toUpperCase());
      resetDetailScroll();
    };

    endpointList.appendChild(li);
  });
  statEndpoints.textContent = count;
}

function syntaxHighlight(json) {
  if (typeof json != 'string') {
    json = JSON.stringify(json, undefined, 2);
  }
  json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function (match) {
    let color = '#a5b4fc'; // default
    if (/^"/.test(match)) {
      if (/:$/.test(match)) {
        color = '#38bdf8'; // keys
      } else {
        color = '#a78bfa'; // strings
      }
    } else if (/true|false/.test(match)) {
      color = '#34d399'; // booleans
    } else if (/null/.test(match)) {
      color = '#f87171'; // null
    } else {
      color = '#fbbf24'; // numbers
    }
    return '<span style="color:' + color + '">' + match + '</span>';
  });
}

function resetDetailScroll() {
  if (detailPanel) {
    detailPanel.scrollTop = 0;
  }
}

function renderDetails(path, method) {
  welcomeState.classList.add('hidden');
  endpointDetails.classList.remove('hidden');
  
  const operation = currentSpec.paths[path][method.toLowerCase()];
  const isWS = isWebSocketOperation(method, operation);
  const displayMethod = displayMethodFor(method, operation);

  elMethod.className = `method-badge large badge-${displayMethod}`;
  elMethod.textContent = displayMethod;
  elPath.textContent = path;

  if (copyPythonBtn) {
    copyPythonBtn.style.display = isWS ? 'none' : '';
  }
  
  elParams.innerHTML = '';
  if (operation.parameters && operation.parameters.length > 0) {
    operation.parameters.forEach(p => {
      const row = document.createElement('div');
      row.className = 'param-row';
      const schemaType = p.schema && p.schema.type ? p.schema.type : 'string';
      
      row.innerHTML = `
        <div class="param-name">${p.name}</div>
        <div class="param-in">${p.in}</div>
        <div class="param-type">${schemaType}</div>
      `;
      elParams.appendChild(row);
    });
  } else {
    elParams.innerHTML = `<div style="color: var(--text-muted); font-size: 0.9rem; padding: 1rem;">${isWS ? 'No upgrade query params or Sec-WebSocket headers captured yet.' : 'No parameters detected.'}</div>`;
  }

  if (isWS) {
    if (tabSchema) tabSchema.textContent = 'Message Schema';
    if (tabRaw) tabRaw.textContent = 'Frame Log';

    const summary = operation.summary || 'WebSocket Connection';
    const description = operation.description || 'Detected WebSocket upgrade on this endpoint.';
    const stats = operation['x-websocket-stats'];
    const schemaBlock = renderWebSocketSchemas(operation);

    elResponse.innerHTML = `
      <div style="padding: 0.5rem 0;">
        <div style="color: #f472b6; font-weight: 600; margin-bottom: 0.75rem;">${summary}</div>
        <div style="color: var(--text-muted); line-height: 1.6;">${description}</div>
        <div style="margin-top: 1.25rem;">
          <div style="color: #38bdf8; font-size: 0.85rem; margin-bottom: 0.5rem;">Live Frame Stats</div>
          ${renderWebSocketStats(stats)}
        </div>
        ${schemaBlock}
      </div>
    `;

    elRaw.innerHTML = renderWebSocketFrameLog(operation['x-websocket-frames']);
    return;
  }

  if (tabSchema) tabSchema.textContent = 'JSON Schema';
  if (tabRaw) tabRaw.textContent = 'Last Raw Payload';
  
  const response = operation.responses && operation.responses['200'];
  if (response && response.content && response.content['application/json']) {
    const schema = response.content['application/json'].schema;
    elResponse.innerHTML = syntaxHighlight(schema);
  } else {
    elResponse.innerHTML = '<span style="color: #64748b;">// No JSON response payload intercepted yet.</span>';
  }

  if (operation['x-last-payload']) {
    elRaw.innerHTML = syntaxHighlight(operation['x-last-payload']);
  } else {
    elRaw.innerHTML = '<span style="color: #64748b;">// No raw payload captured.</span>';
  }
}

// Export logic
exportBtn.addEventListener('click', () => {
  if (!currentSpec) return;
  const blob = new Blob([JSON.stringify(currentSpec, null, 2)], { type: 'application/json' });
  downloadBlob(blob, `shadowschema_${sessionExportSlug()}.json`);
});

if (exportYamlBtn) {
  exportYamlBtn.addEventListener('click', async () => {
    try {
      const res = await fetch(`${API_URL}/export-map?format=yaml`);
      if (!res.ok) throw new Error('YAML export failed');
      const blob = await res.blob();
      downloadBlob(blob, `shadowschema_${sessionExportSlug()}.yaml`);
    } catch (err) {
      console.error(err);
    }
  });
}

function setSdkButtonsDisabled(disabled) {
  Object.values(sdkButtons).forEach(btn => {
    if (btn) btn.disabled = disabled;
  });
}

function downloadSdk(language) {
  const btn = sdkButtons[language];
  if (!btn) return;
  const originalText = btn.textContent;
  btn.textContent = '⏳ Generating...';
  btn.disabled = true;

  fetch(`${API_URL}/generate-sdk`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ language })
  })
  .then(res => {
    if (!res.ok) throw new Error("Failed to generate SDK");
    return res.blob();
  })
  .then(blob => {
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `shadowschema_${language}_sdk.zip`;
    a.click();
    window.URL.revokeObjectURL(url);
    btn.textContent = '✅ Success';
    setTimeout(() => { btn.textContent = originalText; btn.disabled = false; }, 2000);
  })
  .catch(err => {
    console.error(err);
    btn.textContent = '❌ Error';
    setTimeout(() => { btn.textContent = originalText; btn.disabled = false; }, 2000);
  });
}

Object.entries(sdkButtons).forEach(([language, btn]) => {
  if (btn) btn.addEventListener('click', () => downloadSdk(language));
});

// Copy Python Script logic
if (copyPythonBtn) {
  copyPythonBtn.addEventListener('click', async () => {
    if (!selectedPath || !selectedMethod || !currentSpec) return;

    const operation = currentSpec.paths[selectedPath][selectedMethod.toLowerCase()];

    let baseUrl = currentSpec.servers && currentSpec.servers.length > 0 ? currentSpec.servers[0].url : "https://target-domain.com";
    if (baseUrl === "/") {
      baseUrl = "https://" + currentSpec.info.title;
    }

    const url = baseUrl + selectedPath;
    const vaultHeaders = await resolveVaultHeaders(currentSpec);
    const headers = {
      "User-Agent": "ShadowSchema-Replay/1.0",
      ...vaultHeaders,
    };

    let pythonScript = `import requests\nimport json\n\nurl = "${url}"\n\nheaders = ${JSON.stringify(headers, null, 4)}\n\n`;

    let payloadKwarg = "";
    if (['POST', 'PUT', 'PATCH'].includes(selectedMethod) && operation['x-last-payload']) {
      pythonScript += `payload = ${JSON.stringify(operation['x-last-payload'], null, 4)}\n\n`;
      payloadKwarg = ", json=payload";
    }

    const vaultNote = Object.keys(vaultHeaders).length > 0
      ? `# Auth headers auto-injected from ShadowSchema Auth Vault\n`
      : `# No Auth Vault credentials captured yet for this session\n`;

    pythonScript = vaultNote + pythonScript;
    pythonScript += `response = requests.request("${selectedMethod}", url, headers=headers${payloadKwarg})\n\nprint(f"Status: {response.status_code}")\nprint(response.text)\n`;

    navigator.clipboard.writeText(pythonScript).then(() => {
      const originalText = copyPythonBtn.textContent;
      copyPythonBtn.textContent = Object.keys(vaultHeaders).length > 0 ? "✅ Copied w/ Vault" : "✅ Copied!";
      setTimeout(() => {
        copyPythonBtn.textContent = originalText;
      }, 2000);
    });
  });
}

// Event Listeners
sessionSelect.addEventListener('change', (e) => {
  switchSession(e.target.value);
});

newSessionBtn.addEventListener('click', () => {
  modal.classList.remove('hidden');
  inputName.value = '';
  inputTarget.value = '';
  inputName.focus();
});

btnCancel.addEventListener('click', () => {
  modal.classList.add('hidden');
});

// Admin Modal Logic
manageBtn.addEventListener('click', async () => {
  adminModal.classList.remove('hidden');
  await renderAdminList();
});

adminClose.addEventListener('click', () => {
  adminModal.classList.add('hidden');
});

async function renderAdminList() {
  adminList.innerHTML = '';
  try {
    const res = await fetch(`${API_URL}/sessions`);
    const sessions = await res.json();
    
    sessions.forEach(s => {
      const li = document.createElement('li');
      li.className = 'endpoint-item';
      li.style.justifyContent = 'space-between';
      li.style.alignItems = 'center';
      li.style.gap = '0.75rem';

      const info = document.createElement('div');
      info.style.display = 'flex';
      info.style.flexDirection = 'column';
      info.style.gap = '4px';
      info.style.flex = '1';

      const title = document.createElement('strong');
      title.style.color = 'var(--accent-cyan)';
      title.textContent = s.name;

      const target = document.createElement('span');
      target.style.fontSize = '0.8rem';
      target.style.color = 'var(--text-muted)';
      target.textContent = `Target: ${s.target}`;

      const updated = document.createElement('span');
      updated.style.fontSize = '0.75rem';
      updated.style.color = 'var(--text-muted)';
      updated.textContent = s.updated_at
        ? `Updated: ${new Date(s.updated_at).toLocaleString()}`
        : 'Updated: —';

      info.append(title, target, updated);

      const actions = document.createElement('div');
      actions.style.display = 'flex';
      actions.style.gap = '0.5rem';

      const renameBtn = document.createElement('button');
      renameBtn.className = 'glass-btn small';
      renameBtn.textContent = 'Rename';
      renameBtn.onclick = async () => {
        const newName = prompt('Rename session:', s.name);
        if (!newName) return;
        const trimmed = newName.trim();
        if (!trimmed || trimmed === s.name) return;

        renameBtn.textContent = '...';
        const res = await fetch(`${API_URL}/sessions/rename`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ id: s.id, name: trimmed }),
        });
        if (!res.ok) {
          renameBtn.textContent = 'Rename';
          return;
        }
        if (s.id.toString() === currentSessionId) {
          currentSessionName = trimmed;
        }
        await fetchSpec();
        await renderAdminList();
      };

      const delBtn = document.createElement('button');
      delBtn.className = 'glass-btn small';
      delBtn.style.background = 'rgba(239, 68, 68, 0.2)';
      delBtn.style.borderColor = 'rgba(239,68,68,0.4)';
      delBtn.style.color = '#f87171';
      delBtn.textContent = 'Delete';
      delBtn.onclick = async () => {
        delBtn.textContent = '...';
        await fetch(`${API_URL}/sessions/delete`, {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({id: s.id})
        });
        currentSessionId = null;
        currentSessionName = null;
        await fetchSpec();
        await renderAdminList();
      };

      actions.append(renameBtn, delBtn);
      li.append(info, actions);
      adminList.appendChild(li);
    });
  } catch(err) {
    console.error(err);
  }
}

// Discovered Domains Modal Logic
function showDiscoveredPlaceholder(message) {
  discoveredList.innerHTML = '';
  const li = document.createElement('li');
  li.className = 'endpoint-item';
  li.style.justifyContent = 'center';
  li.style.color = 'var(--text-muted)';
  li.style.padding = '1rem';
  li.textContent = message;
  discoveredList.appendChild(li);
}

async function renderDiscoveredList() {
  if (!discoveredList) return;

  showDiscoveredPlaceholder('Loading shadow domains...');
  try {
    const res = await fetch(`${API_URL}/discovered`);
    if (!res.ok) {
      showDiscoveredPlaceholder('Failed to load shadow domains.');
      return;
    }

    const domains = await res.json();
    if (!Array.isArray(domains)) {
      showDiscoveredPlaceholder('Unexpected response from server.');
      return;
    }

    if (statDomains) statDomains.textContent = domains.length;
    discoveredList.innerHTML = '';

    if (domains.length === 0) {
      showDiscoveredPlaceholder('No out-of-scope domains detected yet. Route traffic through the proxy to discover shadow domains.');
      return;
    }

    domains.forEach(d => {
      const li = document.createElement('li');
      li.className = 'endpoint-item';
      li.style.justifyContent = 'space-between';
      li.innerHTML = `
        <span style="font-family: var(--font-mono); color: var(--text-main);">${d}</span>
        <button class="glass-btn small primary">+ Add to Scope</button>
      `;

      const addBtn = li.querySelector('button');
      addBtn.onclick = async () => {
        addBtn.textContent = '...';
        await fetch(`${API_URL}/sessions/add-target`, {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({domain: d})
        });
        await fetchSpec();
        await renderDiscoveredList();
      };

      discoveredList.appendChild(li);
    });
  } catch(err) {
    console.error(err);
    showDiscoveredPlaceholder('Failed to load shadow domains.');
  }
}

if (viewDomainsBtn && discoveredModal && ddClose && discoveredList) {
  viewDomainsBtn.addEventListener('click', async () => {
    discoveredModal.classList.remove('hidden');
    await renderDiscoveredList();
  });

  ddClose.addEventListener('click', () => {
    discoveredModal.classList.add('hidden');
  });
}

async function fetchDiscovered() {
  try {
    const res = await fetch(`${API_URL}/discovered`);
    if (!res.ok) return;
    const domains = await res.json();
    if (Array.isArray(domains) && statDomains) {
      statDomains.textContent = domains.length;
    }
  } catch(err){}
}

btnCreate.addEventListener('click', async () => {
  const name = inputName.value.trim();
  const target = inputTarget.value.trim();
  const ignore_rules = inputIgnore.value.trim();
  if (!name || !target) return;

  try {
    await fetch(`${API_URL}/sessions`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({name, target, ignore_rules})
    });
    modal.classList.add('hidden');
    selectedPath = null;
    selectedMethod = null;
    currentSpec = null;
    welcomeState.classList.remove('hidden');
    endpointDetails.classList.add('hidden');
    await fetchSpec();
  } catch(err) {
    console.error(err);
  }
});

setInterval(() => {
  fetchSpec();
  fetchDiscovered();
}, 2000);
fetchSpec();
fetchDiscovered();
