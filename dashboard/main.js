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
const elRaw = document.getElementById('endpoint-raw');
const copyPythonBtn = document.getElementById('copy-python-btn');
const exportBtn = document.getElementById('export-json-btn');
const exportYamlBtn = document.getElementById('export-yaml-btn');
const genSdkPythonBtn = document.getElementById('gen-sdk-python-btn');
const genSdkTsBtn = document.getElementById('gen-sdk-ts-btn');

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

// Vault logic
if (vaultBtn && vaultModal && vaultClose && vaultList) {
  vaultBtn.addEventListener('click', () => {
    vaultModal.classList.remove('hidden');
    vaultList.innerHTML = '<tr><td colspan="3" style="text-align:center; padding: 1rem;">Loading...</td></tr>';
    
    fetch('http://localhost:38081/vault')
      .then(res => res.json())
      .then(creds => {
        vaultList.innerHTML = '';
        if (!creds || creds.length === 0) {
          vaultList.innerHTML = '<tr><td colspan="3" style="text-align:center; padding: 1rem;">No credentials captured yet.</td></tr>';
          return;
        }
        creds.forEach(c => {
          const tr = document.createElement('tr');
          tr.style.borderBottom = '1px solid rgba(255,255,255,0.05)';
          tr.innerHTML = `
            <td style="padding: 0.75rem 0.5rem; font-family: var(--font-mono); color: var(--accent-cyan);">${c.header_name}</td>
            <td style="padding: 0.75rem 0.5rem; font-family: var(--font-mono); word-break: break-all;">${c.token_value}</td>
            <td style="padding: 0.75rem 0.5rem; font-size: 0.85rem; color: var(--text-muted);">${new Date(c.first_seen).toLocaleString()}</td>
          `;
          vaultList.appendChild(tr);
        });
      })
      .catch(err => {
        vaultList.innerHTML = `<tr><td colspan="3" style="text-align:center; padding: 1rem; color: red;">Error: ${err}</td></tr>`;
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
    if(exportYamlBtn) exportYamlBtn.disabled = false;
    if(genSdkPythonBtn) genSdkPythonBtn.disabled = false;
    if(genSdkTsBtn) genSdkTsBtn.disabled = false;
    
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
    if(genSdkPythonBtn) genSdkPythonBtn.disabled = true;
    if(genSdkTsBtn) genSdkTsBtn.disabled = true;
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

  statRoutes.textContent = Object.keys(currentSpec.paths).length;

  Object.entries(currentSpec.paths).forEach(([path, methods]) => {
    Object.keys(methods).forEach(method => {
      count++;
      const li = document.createElement('li');
      li.className = 'endpoint-item';
      if (path === selectedPath && method === selectedMethod.toLowerCase()) {
        li.classList.add('active');
      }
      
      let displayMethod = method.toUpperCase() === 'TRACE' ? 'WS' : method.toUpperCase();
      
      li.innerHTML = `
        <span class="method-badge badge-${displayMethod}">${displayMethod}</span>
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
  
  let displayMethod = method.toUpperCase() === 'TRACE' ? 'WS' : method.toUpperCase();
  elMethod.className = `method-badge large badge-${displayMethod}`;
  elMethod.textContent = displayMethod;
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

if (exportYamlBtn) {
  exportYamlBtn.addEventListener('click', () => {
    window.location.href = 'http://localhost:38081/export-map?format=yaml';
  });
}

function downloadSdk(language) {
  const btn = language === 'python' ? genSdkPythonBtn : genSdkTsBtn;
  const originalText = btn.textContent;
  btn.textContent = '⏳ Generating...';
  btn.disabled = true;

  fetch('http://localhost:38081/generate-sdk', {
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

if (genSdkPythonBtn) {
  genSdkPythonBtn.addEventListener('click', () => downloadSdk('python'));
}
if (genSdkTsBtn) {
  genSdkTsBtn.addEventListener('click', () => downloadSdk('typescript-fetch'));
}

// Copy Python Script logic
if (copyPythonBtn) {
  copyPythonBtn.addEventListener('click', () => {
    if (!selectedPath || !selectedMethod || !currentSpec) return;

    const operation = currentSpec.paths[selectedPath][selectedMethod.toLowerCase()];
    
    // Get the base URL from the spec servers if available, or just use a placeholder
    let baseUrl = currentSpec.servers && currentSpec.servers.length > 0 ? currentSpec.servers[0].url : "https://target-domain.com";
    if (baseUrl === "/") {
      baseUrl = "https://" + currentSpec.info.title; // fallback
    }
    
    let url = baseUrl + selectedPath;
    let headers = {
      "User-Agent": "ShadowSchema-Replay/1.0"
    };

    let pythonScript = `import requests\nimport json\n\nurl = "${url}"\n\nheaders = ${JSON.stringify(headers, null, 4)}\n\n`;

    let payloadKwarg = "";
    if (['POST', 'PUT', 'PATCH'].includes(selectedMethod) && operation['x-last-payload']) {
      pythonScript += `payload = ${JSON.stringify(operation['x-last-payload'], null, 4)}\n\n`;
      payloadKwarg = ", json=payload";
    }

    pythonScript += `response = requests.request("${selectedMethod}", url, headers=headers${payloadKwarg})\n\nprint(f"Status: {response.status_code}")\nprint(response.text)\n`;

    navigator.clipboard.writeText(pythonScript).then(() => {
      const originalText = copyPythonBtn.textContent;
      copyPythonBtn.textContent = "✅ Copied!";
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
