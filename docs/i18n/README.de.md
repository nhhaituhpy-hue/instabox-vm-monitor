<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Deutsch)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <strong>Deutsch</strong> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Leichtgewichtiges Go-Dashboard für Multi-Account-**instabox.sh**-VMs mit 24/7-Keep-Awake-Verwaltung und Remote-MCP-Orchestrierung.

## Funktionen

- **Einzelne schlanke Go-Binary** (`CGO_ENABLED=0`, eingebettetes Web-UI über `embed.FS`, Pure-Go-SQLite `modernc.org/sqlite`).
- **Admin-Ersteinrichtung**: Einmalige Registrierung beim ersten Start (`scrypt`, `HttpOnly`-Cookies, CSRF- & Fetch-Metadata-Schutz).
- **Multi-Account-Verwaltung & 2-Tab-VM-Erstellung**: Verknüpfen Sie Konten via AES-256-GCM-verschlüsseltem API-Key (`ibk_...`) oder erstellen Sie neue VMs direkt anhand von Live-Kontingenten, Templates/Snapshots und Ressourcenprofilen.
- **Inline-Umbenennung, bestätigtes Löschen & Raster-/Kartenansicht**: Farbige Statuspunkte (korrekte Behandlung von `runtime.ready=false` nach Snapshot-Resume), bündige Desktop-Kartenansicht, direkte Namensbearbeitung und Löschdialog mit Bestätigung.
- **Pro-VM-Keep-Awake & Auto-Zyklus (`keepalive`)**: Erneuert automatisch `ibmon-keepalive`-Leases (3600 s), weckt Idle-VMs nach Cooldown, führt präventive Neustarts vor dem 8h-/`maxRunMinutes`-Limit durch und respektiert manuelle Stopps.
- **4 SVG-Aktionsbuttons mit Hover-Tooltip**: Aufwecken, Pausieren, Starten und sofortiges Stoppen über die offizielle `instabox.sh`-API.
- **Echtzeit-Laufzeituhr & Beszel-ähnliche Diagramme**: Live-Timer ab `runStartedAt`, Monatsverbrauch sowie CPU-, RAM-, Festplatten- (`/` & `/data`), Netzwerk- und Disk-I/O-Graphen (30s-Intervall, 24h-Backfill, 7 Tage Aufbewahrung).
- **Service-API-Keys (`ibmon_*`) & Remote-MCP (`/mcp`)**: Erstellen und widerrufen Sie SHA-256-gehashte Bearer-Tokens unter **Menü → API-Keys & MCP**, damit Hermes Agent oder Skripte alle VMs über 12 MCP-Tools und REST-Orchestrierung steuern können, ohne `ibk_*`-Schlüssel preiszugeben. Das Löschen von VMs per API/MCP erfordert zwingend ein 5 Minuten gültiges Einmal-Bestätigungstoken.
- **PWA & 11 Sprachen**: Google-Sans-Typografie, Hell/Dunkel/System-Design, Logo/Icons/Manifest.

## Schnellstart (Docker)

```bash
docker compose up -d --build
```
