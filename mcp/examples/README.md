# MCP example configs

Copy these templates into the config location for your coding agent. Adjust paths if you clone ShadowSchema somewhere other than the default location.

| File | Copy to |
|------|---------|
| `grok-config.toml` | `~/.grok/config.toml` (append `[mcp_servers.shadowschema]`) |
| `grok-project-config.toml` | `.grok/config.toml` in your project repo |
| `opencode.jsonc` | `opencode.json` / `opencode.jsonc` (merge `mcp` block) |
| `cursor-mcp.json` | `.cursor/mcp.json` in your project |
| `claude-code.json` | `mcpServers` block in `~/.claude.json` |
| `claude-desktop.json` | `claude_desktop_config.json` (merge `mcpServers`) |
| `vscode-mcp.json` | VS Code user `mcp.json` |
| `mcp.json` | `.mcp.json` at project root (portable) |
| `dev-from-repo.json` | Use while developing from a local clone (before npm publish) |

## Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SHADOWSCHEMA_EXPORT_URL` | `http://localhost:38081` | Export API base URL |
| `SHADOWSCHEMA_PROXY_URL` | `http://127.0.0.1:38080` | MITM proxy (for browser MCP setup docs) |

No secrets are required for the MCP server itself. Auth vault data is read from your local ShadowSchema instance only.

**Note:** Examples reference `@notfixingit3/shadowschema-mcp` for future npm publish. Until then, use [`dev-from-repo.json`](dev-from-repo.json) with `npm start`.