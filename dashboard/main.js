import './style.css';
import { registerSW } from 'virtual:pwa-register';

registerSW({ immediate: true });

const API_URL = 'http://localhost:38081';

const statusText = document.getElementById('connection-status');
const pulse = document.querySelector('.pulse');
const endpointList = document.getElementById('endpoint-list');
const welcomeState = document.getElementById('welcome-state');
const endpointDetails = document.getElementById('endpoint-details');

const elMethod = document.getElementById('endpoint-method');
const elPath = document.getElementById('endpoint-path');
const elParams = document.getElementById('endpoint-params');
const elResponse = document.getElementById('endpoint-response');
const exportBtn = document.getElementById('export-json-btn');

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

// Discovered Domains
const viewDomainsBtn = document.getElementById('view-domains-btn');
const discoveredModal = document.getElementById('discovered-domains-modal');
const ddClose = document.getElementById('dd-close');
const discoveredList = document.getElementById('discovered-admin-list');

// Metric Elements
const statEndpoints = document.getElementById('stat-endpoints');
const statDomains = document.getElementById('stat-domains');
const statTargets = document.getElementById('stat-targets');

// UI elements
const searchInput = document.getElementById('endpoint-search');
const tabSchema = document.getElementById('tab-schema');
const tabRaw = document.getElementById('tab-raw');
const elRaw = document.getElementById('endpoint-raw');

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

// Search Logic
searchInput.addEventListener('input', (e) => {
  const query = e.target.value.toLowerCase();
  const items = endpointList.querySelectorAll('.endpoint-item');
  items.forEach(li => {
    const text = li.textContent.toLowerCase();
    if (text.includes(query)) {
      li.style.display = 'flex';
    } else {
      li.style.display = 'none';
    }
  });
});

let currentSpec = null;
let selectedPath = null;
let selectedMethod = null;
let currentSessionId = null;

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
  }
}

function renderSidebar() {
  endpointList.innerHTML = '';
  let count = 0;
  
  if (!currentSpec || !currentSpec.paths) {
    statEndpoints.textContent = "0";
    return;
  }

  Object.entries(currentSpec.paths).forEach(([path, methods]) => {
    Object.keys(methods).forEach(method => {
      count++;
      const li = document.createElement('li');
      li.className = 'endpoint-item';
      if (path === selectedPath && method === selectedMethod.toLowerCase()) {
        li.classList.add('active');
      }
      
      li.innerHTML = `
        <span class="method-badge badge-${method.toUpperCase()}">${method.toUpperCase()}</span>
        <span class="endpoint-path-label">${path}</span>
      `;
      
      li.onclick = () => {
        selectedPath = path;
        selectedMethod = method.toUpperCase();
        renderSidebar(); 
        renderDetails(path, method.toUpperCase());
      };
      
      endpointList.appendChild(li);
    });
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

function renderDetails(path, method) {
  welcomeState.classList.add('hidden');
  endpointDetails.classList.remove('hidden');
  
  elMethod.className = `method-badge large badge-${method}`;
  elMethod.textContent = method;
  elPath.textContent = path;
  
  const operation = currentSpec.paths[path][method.toLowerCase()];
  
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
    elParams.innerHTML = '<div style="color: var(--text-muted); font-size: 0.9rem; padding: 1rem;">No parameters detected.</div>';
  }
  
  const response = operation.responses['200'];
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
  const dataStr = "data:text/json;charset=utf-8," + encodeURIComponent(JSON.stringify(currentSpec, null, 2));
  const dlAnchorElem = document.createElement('a');
  dlAnchorElem.setAttribute("href", dataStr);
  dlAnchorElem.setAttribute("download", `shadowschema_${currentSessionId}.json`);
  dlAnchorElem.click();
});

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
      li.innerHTML = `
        <div style="display: flex; flex-direction: column; gap: 4px;">
          <strong style="color: var(--accent-cyan)">${s.name}</strong>
          <span style="font-size: 0.8rem; color: var(--text-muted)">Target: ${s.target}</span>
        </div>
        <button class="glass-btn small" style="background: rgba(239, 68, 68, 0.2); border-color: rgba(239,68,68,0.4); color: #f87171;">Delete</button>
      `;
      
      const delBtn = li.querySelector('button');
      delBtn.onclick = async () => {
        delBtn.textContent = '...';
        await fetch(`${API_URL}/sessions/delete`, {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({id: s.id})
        });
        currentSessionId = null; // force a clean reload
        await fetchSpec();
        await renderAdminList();
      };
      
      adminList.appendChild(li);
    });
  } catch(err) {
    console.error(err);
  }
}

// Discovered Domains Modal Logic
viewDomainsBtn.addEventListener('click', async () => {
  discoveredModal.classList.remove('hidden');
  await renderDiscoveredList();
});

ddClose.addEventListener('click', () => {
  discoveredModal.classList.add('hidden');
});

async function renderDiscoveredList() {
  discoveredList.innerHTML = '';
  try {
    const res = await fetch(`${API_URL}/discovered`);
    const domains = await res.json();
    
    statDomains.textContent = domains.length;

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
  }
}

async function fetchDiscovered() {
  try {
    const res = await fetch(`${API_URL}/discovered`);
    const domains = await res.json();
    statDomains.textContent = domains.length;
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
