import './style.css';
import { registerSW } from 'virtual:pwa-register';
import { escapeHtml, syntaxHighlight } from './modules/highlight.js';
import { isGraphQLOperation, renderGraphQLPanel } from './modules/graphql.js';
import {
  displayMethodFor,
  isWebSocketOperation,
  renderWebSocketFrameLog,
  renderWebSocketSchemas,
  renderWebSocketStats,
} from './modules/websocket.js';
import {
  resolveVaultHeaders,
  fetchVault,
  renderVaultRows,
  vaultErrorRow,
} from './modules/vault.js';

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

// HAR import
const importHarBtn = document.getElementById('import-har-btn');
const importHarInput = document.getElementById('import-har-input');
if (importHarBtn && importHarInput) {
  importHarBtn.addEventListener('click', () => importHarInput.click());
  importHarInput.addEventListener('change', async () => {
    const file = importHarInput.files && importHarInput.files[0];
    importHarInput.value = '';
    if (!file) return;
    const original = importHarBtn.textContent;
    importHarBtn.textContent = 'Importing…';
    importHarBtn.disabled = true;
    try {
      const text = await file.text();
      const res = await fetch(`${API_URL}/import-har?only_matching_target=true`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: text,
      });
      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `HTTP ${res.status}`);
      }
      const result = await res.json();
      importHarBtn.textContent = `✅ ${result.imported || 0}`;
      await fetchSpec();
      setTimeout(() => {
        importHarBtn.textContent = original;
        importHarBtn.disabled = false;
      }, 2500);
    } catch (err) {
      console.error(err);
      importHarBtn.textContent = '❌ Error';
      alert('HAR import failed: ' + (err.message || err));
      setTimeout(() => {
        importHarBtn.textContent = original;
        importHarBtn.disabled = false;
      }, 2500);
    }
  });
}

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


const tabGraphql = document.getElementById('tab-graphql');
const elGraphql = document.getElementById('endpoint-graphql');

function activateDetailTab(which) {
  const tabs = [tabSchema, tabRaw, tabGraphql].filter(Boolean);
  const panes = [
    { el: elResponse, key: 'schema' },
    { el: elRaw, key: 'raw' },
    { el: elGraphql, key: 'graphql' },
  ];
  tabs.forEach((t) => t.classList.toggle('active', t && t.id === `tab-${which === 'schema' ? 'schema' : which === 'raw' ? 'raw' : 'graphql'}`));
  if (tabSchema) tabSchema.classList.toggle('active', which === 'schema');
  if (tabRaw) tabRaw.classList.toggle('active', which === 'raw');
  if (tabGraphql) tabGraphql.classList.toggle('active', which === 'graphql');
  panes.forEach(({ el, key }) => {
    if (!el) return;
    el.classList.toggle('hidden', key !== which);
  });
}

if (tabSchema) tabSchema.addEventListener('click', () => activateDetailTab('schema'));
if (tabRaw) tabRaw.addEventListener('click', () => activateDetailTab('raw'));
if (tabGraphql) tabGraphql.addEventListener('click', () => activateDetailTab('graphql'));

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
  vaultBtn.addEventListener('click', async () => {
    vaultModal.classList.remove('hidden');
    vaultList.innerHTML = '<tr><td colspan="5" style="text-align:center; padding: 1rem;">Loading...</td></tr>';
    try {
      const creds = await fetchVault(API_URL, { includeValues: true });
      renderVaultRows(vaultList, creds);
    } catch (err) {
      vaultErrorRow(vaultList, err);
    }
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
    if (tabGraphql) tabGraphql.classList.add('hidden');
    if (elGraphql) elGraphql.innerHTML = '';
    activateDetailTab('schema');
    return;
  }

  if (tabSchema) tabSchema.textContent = 'JSON Schema';
  if (tabRaw) tabRaw.textContent = 'Last Raw Payload';

  const isGQL = isGraphQLOperation(operation);
  if (tabGraphql) {
    tabGraphql.classList.toggle('hidden', !isGQL);
    tabGraphql.textContent = isGQL ? 'GraphQL' : 'GraphQL';
  }
  if (elGraphql) {
    elGraphql.innerHTML = isGQL ? renderGraphQLPanel(operation) : '';
  }
  // Prefer schema tab; if GraphQL-only view was active and no longer GQL, reset.
  if (!isGQL && tabGraphql && tabGraphql.classList.contains('active')) {
    activateDetailTab('schema');
  }

  const statusKeys = operation.responses ? Object.keys(operation.responses).sort() : [];
  const preferredStatus = statusKeys.includes('200') ? '200' : statusKeys[0];
  const response = preferredStatus && operation.responses[preferredStatus];
  if (response && response.content && response.content['application/json']) {
    const schema = response.content['application/json'].schema;
    elResponse.innerHTML = (statusKeys.length > 1
      ? `<div style="color: #38bdf8; font-size: 0.8rem; margin-bottom: 0.5rem;">Statuses: ${statusKeys.map(escapeHtml).join(', ')} (showing ${escapeHtml(preferredStatus)})</div>`
      : '') + syntaxHighlight(schema);
  } else {
    elResponse.innerHTML = '<span style="color: #64748b;">// No JSON response payload intercepted yet.</span>';
  }

  if (operation['x-last-payload']) {
    let raw = syntaxHighlight(operation['x-last-payload']);
    if (operation['x-last-request-body']) {
      raw = `<div style="color: #38bdf8; font-size: 0.8rem; margin-bottom: 0.35rem;">Request body</div>${syntaxHighlight(operation['x-last-request-body'])}
             <div style="color: #38bdf8; font-size: 0.8rem; margin: 1rem 0 0.35rem;">Response body</div>${raw}`;
    }
    elRaw.innerHTML = raw;
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
    const preferredHost = (currentSpec.servers && currentSpec.servers[0] && currentSpec.servers[0].url)
      ? currentSpec.servers[0].url.replace(/^https?:\/\//, '').split('/')[0]
      : undefined;
    const vaultHeaders = await resolveVaultHeaders(API_URL, currentSpec, preferredHost);
    const headers = {
      "User-Agent": "ShadowSchema-Replay/1.0",
      ...vaultHeaders,
    };

    let pythonScript = `import requests\nimport json\n\nurl = "${url}"\n\nheaders = ${JSON.stringify(headers, null, 4)}\n\n`;

    let payloadKwarg = "";
    // Prefer captured request body — never use response x-last-payload as request body.
    if (['POST', 'PUT', 'PATCH'].includes(selectedMethod) && operation['x-last-request-body']) {
      pythonScript += `payload = ${JSON.stringify(operation['x-last-request-body'], null, 4)}\n\n`;
      payloadKwarg = ", json=payload";
    } else if (['POST', 'PUT', 'PATCH'].includes(selectedMethod)) {
      pythonScript += `# No request body captured for this operation yet\n\n`;
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
