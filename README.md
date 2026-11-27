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
- **Multi-Account Management & 2-Tab Add Machine**: Bind multiple `instabox.sh` accounts (`ibk_...`, encrypted at rest with AES-256-GCM and never returned to the browser) or create new VMs directly across saved accounts with live quota/capacity inspection, template/snapshot selection, specs, idle policy (whole minutes 5-30 or 0=never on paid plans), `maxRunMinutes`, and `deleteAfterSeconds`/`archiveSeconds` hard lifetime deadlines (the retired `timeout`/`onTimeout` fields are never sent). Fork any VM's disks into a new machine in seconds (`POST /api/machines/{accountId}/{machineId}/fork`) and resize stopped VMs (`vcpus`/`memMiB` via PATCH, optionally `apply: restart`).
- **Inline Rename, Confirmed Delete & Card/Grid Views**: Color-coded live status dots (`running`, `booting`, `idle-paused`, `paused`, `stopped`, `error/cooldown` — accurately handling `runtime.ready=false` on resumed/headless VMs), aligned desktop Card View & Grid View, inline rename (`PATCH`), and confirmation-gated VM deletion.
- **Interactive Console Terminal (`xterm.js` over WebSocket)**: Open a live PTY (`xterm-256color`) terminal window directly for any running or idle-paused VM (`GET /api/machines/{accountId}/{machineId}/terminal`), with persistent named sessions (`main`, `dev`, `logs`, ...), auto-fit resize, fullscreen toggle, shell restart, and mobile quick-key bar—proxied securely through the Go backend without exposing `ibk_*` keys.
- **Per-VM Keep-Awake & Auto-Cycle (`keepalive`)**: Optional per-machine background service that renews `ibmon-keepalive` activity leases (`3600s`), wakes auto-idled VMs after cooldown, and power-cycles before the 8h / `maxRunMinutes` cap while strictly preserving manual stops/suspends.
- **4 Icon-Only SVG Action Buttons**: Wake/Resume, Pause (`suspend`), Start, and immediate Stop (`stop`) via the official `instabox.sh` REST API (`POST /v1/machines/{id}/{wake,resume,suspend,start,stop}` with `Idempotency-Key`), showing descriptive tooltips on hover/focus.
- **Live Runtime Clock & Beszel-style Charts**: Real-time 1-second session uptime clock synced to `instabox.sh` `runStartedAt` + server clock offset, monthly usage totals, and interactive CPU, RAM, Disk (`/data` & `/`), Network, and Disk I/O area charts (30s background polling, 24h backfill, 7-day default retention).
- **Service API Keys (`ibmon_*`) & Remote MCP (`/mcp`)**: Create and revoke SHA-256-hashed bearer tokens from **Menu → API Keys & MCP** so AI agents (such as Hermes Agent) or scripts can orchestrate every saved VM (`exec`, `files`, `ports`, `processes`, `logs`, `connection-info`, `snapshots`, `restore`, lifecycle & keepalive) without ever exposing upstream `ibk_*` account keys. Machine deletion via API key or MCP enforces a mandatory single-use, 5-minute, machine-scoped confirmation challenge before deleting.
- **Modern Web & PWA**: Google Sans typography, automatic + manual Light/Dark/System theme (`color-scheme` + `light-dark()`), custom SVG/PNG icons, `manifest.webmanifest`, service worker, and 11-language UI.

## instabox.sh upstream compatibility (November 2026)

This monitor tracks the current instabox.sh API and encodes these recent upstream semantics:

- **Lifetime timeouts retired (2026-10-20)**: `timeoutSeconds`/`onTimeout`/`timeout` are never sent. Use `deleteAfterSeconds` (hard delete 60s-24h) and `archiveSeconds` (stop, files kept) instead; both show as live countdowns on the VM card.
- **Auto-suspend rules**: paid plans accept whole minutes 300-1800s (5-30 min) or `0` (never); free-plan machines are fixed at 1800s. Invalid values are rejected locally before hitting upstream.
- **Wake semantics**: `/wake` of an idle-paused VM can answer a retryable `409 operation_conflict` naming the in-progress resume; the monitor follows that operation instead of failing the action.
- **Cooldowns**: max-run cooldowns escalate on the free plan (5-20 / 60-75 / 120-135 min for consecutive pauses) and abuse-lock cooldowns (`cooldownReason` `abuse_*`, up to 24h) are labelled distinctly on the card clock.
- **Machine resize**: per-card resize dialog (sliders icon) listing only the sizes the account's plan allows. Resizing a running machine can apply immediately (`apply: restart` — processes die, `/data` persists) or be deferred to the next boot.
- **CI template restriction**: instabox.sh reserves free-plan `ci` boxes for GitHub Actions jobs — direct creates fail upstream with `403 template_job_only`. The create dialog marks the template 'CI jobs only' on free accounts, blocks the submit client-side, and maps the upstream error to an explanation pointing at `runs-on: instabox-medium`. Paid plans can still create `ci` boxes directly. To prepare a CI snapshot you must run jobs through the GitHub App flow; warm snapshots work for private repos only.
- **On-demand CI boxes**: since free-plan `ci` machines only exist as Actions job runners, the monitor drives that mechanism deliberately — the 'CI boxes' dialog configures a GitHub token + repo per account (a private `ci-box` repo is auto-created when left empty), Start dispatches a workflow whose job holds the box in a sleep loop, Stop cancels the run and instabox deletes the box. The user controls exactly when a box starts and ends; the box's machineId is reconciled from the org's instabox github-jobs feed. Session budget is 7h40, under the 8h free-plan cap. Requires a one-time manual step: install the instabox GitHub App on the GitHub account/org that owns the workflow repo.
- **Time-window scheduling**: per-machine daily run windows (fixed UTC offset, default UTC+7) editable on the card, via `PUT /api/machines/{accountId}/{machineId}/schedule` or the MCP `control_machine` tool (`schedule`/`scheduleWindows`/`scheduleTzOffsetMin`). Inside a window a stopped/suspended machine is auto-started or woken; outside windows a running machine is auto-stopped. Windows never cross midnight — split overnight spans (e.g. `22:00-00:00, 00:00-06:00`). Scheduling and keepalive are mutually exclusive: enabling one disables the other. Processes do not survive the scheduled stop — install services with systemd/docker restart policies inside the VM if they must resume. **In-window maintenance:** windows of 29 minutes or longer are automatically kept active for their whole duration (shorter windows end before the 30-minute idle suspension could trigger). Inside a maintained window the monitor runs a tiny in-machine command every ~20 minutes (real activity — each buys the +30 minute run extension), refreshes the activity lease against idle suspension, and power-cycles (stop + start, ~12 s gap) when the run approaches its cap — but never if the window closes within 20 minutes. A 10-hour window therefore spans two run sessions with one brief interruption in the middle.
- **Keepalive uses real activity: since free-plan runs extend only on real use (activity leases no longer count), the per-machine keepalive executes a trivial command in the VM every ~20 minutes instead of just renewing the lease, then proactively stops and restarts the machine before the 8h cap so the escalating max-run cooldown ladder never engages.
- **Fork**: `POST /v1/machines/{id}/fork` copies disks into a new machine in seconds; exposed via REST, MCP (`fork_machine`) and accepts `deleteAfterSeconds`.
- **Resize**: `PATCH /v1/machines/{id}` accepts `vcpus`/`memMiB` for stopped machines, or `apply: restart` for a running/paused machine.

## ci-run.sh — build/test any local repo on a throwaway CI box

`scripts/ci-run.sh` chains the whole loop in one command: start the CI box via the monitor API, push a local repo with `instabox cp`, run any build/test command over `instabox ssh`, pull the artifacts back into `./ci-artifacts/<timestamp>/` — and the box **stays alive** so the next iteration skips the boot (measured: cold round ~110s including boot, repeat rounds ~18s for this repo thanks to the reuse + Go build cache). Pass `STOP=1` to end the session, or press Stop in the dashboard; the box also dies on its own when the ~7h40 job budget runs out.

```bash
MONKEY=ibmon_... IBK=ibk_... ./scripts/ci-run.sh . \
  'go test ./... && CGO_ENABLED=0 go build -o /data/work/dist/server ./cmd/server' \
  /data/work/dist
```

`MONKEY` is a monitor API key (dashboard). `IBK` is the upstream instabox API key of the box-owning account (instabox.sh dashboard → API keys). artifacts land in `./ci-artifacts/`; the box is reused across runs until you pass `STOP=1`. Requires `instabox`, `jq`, `curl` on the host.

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

3. Run `/reload-mcp` (or `hermes mcp test instabox`). Available MCP tools: `list_fleet`, `get_account_options`, `create_machine` (with `deleteAfterSeconds`/`archiveSeconds`), `control_machine` (lifecycle + rename + resize + idle/max-run), `fork_machine`, `delete_machine` *(2-step confirmation gated)*, `exec_command`, `read_files`, `write_file`, `manage_ports`, `inspect_machine`, `manage_snapshots`, and `get_operation`.

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
