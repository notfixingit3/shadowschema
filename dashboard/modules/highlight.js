/** Escape HTML special characters. */
export function escapeHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

/** Pretty-print and colorize JSON for the code window. */
export function syntaxHighlight(json) {
  if (typeof json != 'string') {
    json = JSON.stringify(json, undefined, 2);
  }
  json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  return json.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    function (match) {
      let color = '#a5b4fc';
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          color = '#38bdf8';
        } else {
          color = '#a78bfa';
        }
      } else if (/true|false/.test(match)) {
        color = '#34d399';
      } else if (/null/.test(match)) {
        color = '#f87171';
      } else {
        color = '#fbbf24';
      }
      return '<span style="color:' + color + '">' + match + '</span>';
    },
  );
}
