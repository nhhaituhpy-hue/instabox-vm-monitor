<p align="center">
  <img src="internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor</h1>

<p align="center">
  Lightweight Go &amp; Modern Web monitor, 24/7 keep-awake manager, and Remote MCP orchestration console for multi-account <strong>instabox.sh</strong> cloud sandboxes.
</p>

<p align="center">
  <strong>Languages:</strong>
  <a href="docs/i18n/README.vi.md">Tiếng Việt</a> ·
  <a href="README.md">English</a> ·
  <a href="docs/i18n/README.zh.md">中文</a> ·
  <a href="docs/i18n/README.fr.md">Français</a> ·
  <a href="docs/i18n/README.ja.md">日本語</a> ·
  <a href="docs/i18n/README.ko.md">한국어</a> ·
  <a href="docs/i18n/README.es.md">Español</a> ·
  <a href="docs/i18n/README.pt.md">Português</a> ·
  <a href="docs/i18n/README.de.md">Deutsch</a> ·
  <a href="docs/i18n/README.it.md">Italiano</a> ·
  <a href="docs/i18n/README.ru.md">Русский</a>
</p>

## Features

- **Single lightweight Go binary**: Pure Go (`CGO_ENABLED=0`) with embedded static web UI (`embed.FS`) and embedded pure-Go SQLite storage (`modernc.org/sqlite`), inspired by CLIProxyAPI and Beszel.
- **First-run Admin Setup**: Locks registration after creating the initial administrator account (`scrypt` password hashing, `HttpOnly` session cookies, CSRF + Fetch Metadata protection).
- **Multi-Account Management & 2-Tab Add Machine**: Bind multiple `instabox.sh` accounts (`ibk_...`, encrypted at rest with AES-256-GCM and never returned to the browser) or create new VMs directly across saved accounts with live quota/capacity inspection, template/snapshot selection, specs, idle policy, and `maxRunMinutes`.
- **Inline Rename, Confirmed Delete & Card/Grid Views**: Color-coded live status dots (`running`, `booting`, `idle-paused`, `paused`, `stopped`, `error/cooldown` — accurately handling `runtime.ready=false` on resumed/headless VMs), aligned desktop Card View & Grid View, inline rename (`PATCH`), and confirmation-gated VM deletion.
- **Interactive Console Terminal (`xterm.js` over WebSocket)**: Open a live PTY (`xterm-256color`) terminal window directly for any running or idle-paused VM (`GET /api/machines/{accountId}/{machineId}/terminal`), with persistent named sessions (`main`, `dev`, `logs`, ...), auto-fit resize, fullscreen toggle, shell restart, and mobile quick-key bar—proxied securely through the Go backend without exposing `ibk_*` keys.
- **Per-VM Keep-Awake & Auto-Cycle (`keepalive`)**: Optional per-machine background service that renews `ibmon-keepalive` activity leases (`3600s`), wakes auto-idled VMs after cooldown, and power-cycles before the 8h / `maxRunMinutes` cap while strictly preserving manual stops/suspends.
- **4 Icon-Only SVG Action Buttons**: Wake/Resume, Pause (`suspend`), Start, and immediate Stop (`stop`) via the official `instabox.sh` REST API (`POST /v1/machines/{id}/{wake,resume,suspend,start,stop}` with `Idempotency-Key`), showing descriptive tooltips on hover/focus.
- **Live Runtime Clock & Beszel-style Charts**: Real-time 1-second session uptime clock synced to `instabox.sh` `runStartedAt` + server clock offset, monthly usage totals, and interactive CPU, RAM, Disk (`/data` & `/`), Network, and Disk I/O area charts (30s background polling, 24h backfill, 7-day default retention).
- **Service API Keys (`ibmon_*`) & Remote MCP (`/mcp`)**: Create and revoke SHA-256-hashed bearer tokens from **Menu → API Keys & MCP** so AI agents (such as Hermes Agent) or scripts can orchestrate every saved VM (`exec`, `files`, `ports`, `processes`, `logs`, `connection-info`, `snapshots`, `restore`, lifecycle & keepalive) without ever exposing upstream `ibk_*` account keys. Machine deletion via API key or MCP enforces a mandatory single-use, 5-minute, machine-scoped confirmation challenge before deleting.
- **Modern Web & PWA**: Google Sans typography, automatic + manual Light/Dark/System theme (`color-scheme` + `light-dark()`), custom SVG/PNG icons, `manifest.webmanifest`, service worker, and 11-language UI.

## Quick Start (Docker)

```bash
docker compose up -d --build
```

Open `http://localhost:8080`, create your initial administrator account, and add your first `instabox.sh` alias + API key (`ibk_...`).

## Connecting Hermes Agent (Remote MCP)

1. Sign in to the web dashboard, open **Menu → API Keys & MCP**, and create a new API key (the raw `ibmon_...` token is shown once).
2. Add the Streamable HTTP MCP server to `~/.hermes/config.yaml` (storing the token in `~/.hermes/.env` as `MCP_INSTABOX_API_KEY`):

```yaml
mcp_servers:
  instabox:
    url: "https://your-monitor-host/mcp"
    headers:
      Authorization: "Bearer ${MCP_INSTABOX_API_KEY}"
    timeout: 620
    connect_timeout: 60
    enabled: true
```

3. Run `/reload-mcp` (or `hermes mcp test instabox`). Available MCP tools: `list_fleet`, `get_account_options`, `create_machine`, `control_machine`, `delete_machine` *(2-step confirmation gated)*, `exec_command`, `read_files`, `write_file`, `manage_ports`, `inspect_machine`, `manage_snapshots`, and `get_operation`.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DATA_DIR` | `/data` | Persistent directory for `monitor.db` and `master.key` |
| `APP_MASTER_KEY` | *(auto)* | Optional secret to derive the 32-byte AES-GCM key |
| `POLL_INTERVAL_SECONDS` | `30` | Background polling interval in seconds (min `10`) |
| `METRICS_RETENTION_DAYS` | `7` | Retention window for stored telemetry samples |
| `SESSION_TTL_HOURS` | `168` | Admin session lifetime in hours |
| `INSTABOX_API_URL` | `https://api.instabox.sh` | Upstream HTTPS API base URL |

## Development & Tests

```bash
go test -v ./...
go vet ./...
```
