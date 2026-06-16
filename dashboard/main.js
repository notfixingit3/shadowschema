import './style.css';

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

// Session elements
const sessionSelect = document.getElementById('session-select');
const newSessionBtn = document.getElementById('new-session-btn');
const modal = document.getElementById('new-session-modal');
const btnCancel = document.getElementById('ns-cancel');
const btnCreate = document.getElementById('ns-create');
const inputName = document.getElementById('ns-name');
const inputTarget = document.getElementById('ns-target');

let currentSpec = null;
let selectedPath = null;
let selectedMethod = null;
let currentSessionId = null;

async function fetchSessions() {
  try {
    const res = await fetch(`${API_URL}/sessions`);
    if (!res.ok) return;
    const sessions = await res.json();
    
    // Check if session list changed
    const currentOptions = Array.from(sessionSelect.options).map(o => o.value);
    const newOptions = sessions.map(s => s.id.toString());
    
    if (currentOptions.join() !== newOptions.join()) {
      sessionSelect.innerHTML = '';
      sessions.forEach(s => {
        const opt = document.createElement('option');
        opt.value = s.id;
        opt.textContent = `${s.name} (${s.target})`;
        sessionSelect.appendChild(opt);
      });
      if (sessions.length > 0 && !currentSessionId) {
        currentSessionId = sessions[0].id.toString();
        sessionSelect.value = currentSessionId;
      } else if (currentSessionId) {
        sessionSelect.value = currentSessionId;
      }
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
    currentSpec = null; // force reload
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
    
    statusText.textContent = 'Connected & Listening';
    pulse.classList.remove('error');
    
    if (JSON.stringify(data) !== JSON.stringify(currentSpec)) {
      currentSpec = data;
      renderSidebar();
      if (selectedPath && selectedMethod) {
        renderDetails(selectedPath, selectedMethod);
      }
    }
  } catch (err) {
    statusText.textContent = 'Disconnected';
    pulse.classList.add('error');
  }
}

function renderSidebar() {
  endpointList.innerHTML = '';
  
  if (!currentSpec || !currentSpec.paths) return;

  Object.entries(currentSpec.paths).forEach(([path, methods]) => {
    Object.keys(methods).forEach(method => {
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
}

function renderDetails(path, method) {
  welcomeState.classList.add('hidden');
  endpointDetails.classList.remove('hidden');
  
  elMethod.className = `method-badge badge-${method}`;
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
        <div class="param-in">[${p.in}]</div>
        <div class="param-type">${schemaType}</div>
      `;
      elParams.appendChild(row);
    });
  } else {
    elParams.innerHTML = '<div style="color: var(--text-muted)">No parameters detected.</div>';
  }
  
  const response = operation.responses['200'];
  if (response && response.content && response.content['application/json']) {
    const schema = response.content['application/json'].schema;
    elResponse.textContent = JSON.stringify(schema, null, 2);
  } else {
    elResponse.textContent = '// No JSON response mapped yet';
  }
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

btnCreate.addEventListener('click', async () => {
  const name = inputName.value.trim();
  const target = inputTarget.value.trim();
  if (!name || !target) return;

  try {
    await fetch(`${API_URL}/sessions`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({name, target})
    });
    modal.classList.add('hidden');
    selectedPath = null;
    selectedMethod = null;
    currentSpec = null;
    welcomeState.classList.remove('hidden');
    endpointDetails.classList.add('hidden');
    await fetchSpec(); // Will reload and fetch new session list
  } catch(err) {
    console.error(err);
  }
});

setInterval(fetchSpec, 2000);
fetchSpec();
