<p align="center">
  <img src="internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor</h1>

<p align="center">
  Lightweight Go &amp; Modern Web monitor and control console for multi-account <strong>instabox.sh</strong> cloud sandboxes.
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
- **Multi-Account Management**: Bind multiple `instabox.sh` accounts by alias + API key (`ibk_...`). Keys are encrypted at rest with AES-256-GCM and never returned in plaintext to the browser.
- **VM Grid & Card Views + Point Checker**: Color-coded live status dots (`running`, `starting`, `idle-paused`, `paused`, `stopped`, `error/cooldown`) with Grid/Card layout switcher.
- **4 Icon-Only SVG Action Buttons**: Wake/Resume, Pause (`suspend`), Start, and immediate Stop (`stop`) via the official `instabox.sh` REST API (`POST /v1/machines/{id}/{wake,resume,suspend,start,stop}` with `Idempotency-Key`), showing descriptive tooltips on hover/focus.
- **Live Runtime Clock & Beszel-style Charts**: Real-time 1-second session uptime clock synced to `instabox.sh` `runStartedAt` + server clock offset, monthly usage totals, and interactive CPU, RAM, Disk (`/data` & `/`), Network, and Disk I/O area charts (30s background polling, 7-day default retention).
- **Modern Web & PWA**: Google Sans typography, automatic + manual Light/Dark/System theme (`color-scheme` + `light-dark()`), custom SVG/PNG icons, `manifest.webmanifest`, service worker, and 11-language UI.

## Quick Start (Docker)

```bash
docker compose up -d --build
```

Open `http://localhost:8080`, create your initial administrator account, and add your first `instabox.sh` alias + API key (`ibk_...`).

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
