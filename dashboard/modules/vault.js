import { escapeHtml } from './highlight.js';

/**
 * @param {string} apiUrl
 * @param {{ includeValues?: boolean, host?: string }} [opts]
 */
export async function fetchVault(apiUrl, opts = {}) {
  const params = new URLSearchParams();
  if (opts.includeValues !== false) {
    params.set('include_values', '1');
  }
  if (opts.host) {
    params.set('host', opts.host);
  }
  const qs = params.toString();
  const res = await fetch(`${apiUrl}/vault${qs ? `?${qs}` : ''}`);
  if (!res.ok) throw new Error(`vault ${res.status}`);
  return res.json();
}

export function vaultHeadersFromSpec(spec) {
  const headers = {};
  const vault = spec?.['x-shadowschema-vault'];
  if (!Array.isArray(vault)) return headers;

  vault.forEach((c) => {
    if (c.header_name && c.token_value) {
      headers[c.header_name] = c.token_value;
    }
  });
  return headers;
}

/**
 * Resolve auth headers for replay, optionally preferring a target host.
 */
export async function resolveVaultHeaders(apiUrl, spec, preferredHost) {
  let headers = vaultHeadersFromSpec(spec);
  if (Object.keys(headers).length > 0) {
    return headers;
  }

  try {
    const creds = await fetchVault(apiUrl, {
      includeValues: true,
      host: preferredHost || undefined,
    });
    creds.forEach((c) => {
      if (c.header_name && c.token_value) {
        headers[c.header_name] = c.token_value;
      }
    });
  } catch (err) {
    console.error('Failed to fetch vault credentials', err);
  }
  return headers;
}

/**
 * Render vault table rows into tbody.
 * @param {HTMLElement} vaultList
 * @param {Array<{header_name:string, token_value:string, host?:string, first_seen:string}>} creds
 */
export function renderVaultRows(vaultList, creds) {
  vaultList.innerHTML = '';
  if (!creds || creds.length === 0) {
    vaultList.innerHTML =
      '<tr><td colspan="5" style="text-align:center; padding: 1rem;">No credentials captured yet.</td></tr>';
    return;
  }

  creds.forEach((c) => {
    const tr = document.createElement('tr');
    tr.style.borderBottom = '1px solid rgba(255,255,255,0.05)';

    const hostCell = document.createElement('td');
    hostCell.style.padding = '0.75rem 0.5rem';
    hostCell.style.fontFamily = 'var(--font-mono)';
    hostCell.style.fontSize = '0.8rem';
    hostCell.style.color = 'var(--text-muted)';
    hostCell.textContent = c.host || '(session)';

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
        setTimeout(() => {
          copyBtn.textContent = original;
        }, 1500);
      });
    });
    actionCell.appendChild(copyBtn);

    tr.append(hostCell, headerCell, valueCell, seenCell, actionCell);
    vaultList.appendChild(tr);
  });
}

export function vaultErrorRow(vaultList, err) {
  vaultList.innerHTML = `<tr><td colspan="5" style="text-align:center; padding: 1rem; color: red;">Error: ${escapeHtml(String(err))}</td></tr>`;
}
