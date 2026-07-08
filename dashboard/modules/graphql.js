import { escapeHtml, syntaxHighlight } from './highlight.js';

/**
 * Render expandable GraphQL operations panel for an OpenAPI operation.
 * @param {Record<string, unknown>} operation
 * @returns {string} HTML
 */
export function renderGraphQLPanel(operation) {
  if (!operation || operation['x-graphql'] !== true) {
    return '';
  }

  const ops = operation['x-graphql-operations'];
  if (!ops || typeof ops !== 'object') {
    return `<div class="gql-panel">
      <div class="gql-panel-header">GraphQL</div>
      <div class="gql-panel-empty">GraphQL traffic detected; operations will appear as queries are observed.</div>
    </div>`;
  }

  const entries = Object.entries(ops).sort(([a], [b]) => a.localeCompare(b));
  if (entries.length === 0) {
    return '';
  }

  const cards = entries
    .map(([key, raw]) => {
      const op = raw && typeof raw === 'object' ? raw : {};
      const type = escapeHtml(op.type || 'query');
      const name = escapeHtml(op.operationName || op.name || key);
      const fields = Array.isArray(op.fields)
        ? op.fields.map((f) => escapeHtml(String(f))).join(', ')
        : '';
      const seen = op['x-last-seen']
        ? escapeHtml(new Date(op['x-last-seen']).toLocaleString())
        : '';
      const variables = op['x-last-variables'];
      const response = op['x-last-response'];

      return `
        <details class="gql-op-card">
          <summary>
            <span class="gql-op-type badge-${type}">${type}</span>
            <span class="gql-op-name">${name}</span>
            ${fields ? `<span class="gql-op-fields">${fields}</span>` : ''}
            ${seen ? `<span class="gql-op-seen">${seen}</span>` : ''}
          </summary>
          <div class="gql-op-body">
            ${
              variables !== undefined
                ? `<div class="gql-block"><div class="gql-block-label">Variables</div>${syntaxHighlight(variables)}</div>`
                : ''
            }
            ${
              response !== undefined
                ? `<div class="gql-block"><div class="gql-block-label">Last response</div>${syntaxHighlight(response)}</div>`
                : ''
            }
            ${
              variables === undefined && response === undefined
                ? `<div class="gql-panel-empty">No variables or response captured yet for this operation.</div>`
                : ''
            }
          </div>
        </details>
      `;
    })
    .join('');

  return `
    <div class="gql-panel">
      <div class="gql-panel-header">GraphQL operations (${entries.length})</div>
      <div class="gql-op-list">${cards}</div>
    </div>
  `;
}

export function isGraphQLOperation(operation) {
  return Boolean(operation && operation['x-graphql'] === true);
}
