import './style.css';

const API_URL = 'http://localhost:38081/export-map';

const statusText = document.getElementById('connection-status');
const pulse = document.querySelector('.pulse');
const endpointList = document.getElementById('endpoint-list');
const welcomeState = document.getElementById('welcome-state');
const endpointDetails = document.getElementById('endpoint-details');

const elMethod = document.getElementById('endpoint-method');
const elPath = document.getElementById('endpoint-path');
const elParams = document.getElementById('endpoint-params');
const elResponse = document.getElementById('endpoint-response');

let currentSpec = null;
let selectedPath = null;
let selectedMethod = null;

async function fetchSpec() {
  try {
    const res = await fetch(API_URL);
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

setInterval(fetchSpec, 2000);
fetchSpec();
