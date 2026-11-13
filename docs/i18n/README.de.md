<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Deutsch)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <strong>Deutsch</strong> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Leichtgewichtiges Go-Monitoring- und Steuerungs-Dashboard für Multi-Account-**instabox.sh**-VMs.

## Funktionen

- **Einzelne schlanke Go-Binary** (`CGO_ENABLED=0`, eingebettetes Web-UI über `embed.FS`, Pure-Go-SQLite `modernc.org/sqlite`).
- **Admin-Ersteinrichtung**: Einmalige Registrierung beim ersten Start (`scrypt`, `HttpOnly`-Cookies, CSRF-Schutz).
- **Multi-Account-Verwaltung**: Mehrere Konten mit Alias und verschlüsseltem API-Key (`ibk_...`, AES-256-GCM).
- **Raster- / Kartenansicht & Status-Punktanzeige**: Farbige Statuspunkte und Live-Filter.
- **4 SVG-Aktionsbuttons mit Hover-Tooltip**: Aufwecken, Pausieren, Starten und sofortiges Stoppen über die offizielle `instabox.sh`-API.
- **Echtzeit-Laufzeituhr & Beszel-ähnliche Diagramme**: Live-Timer ab `runStartedAt`, Monatsverbrauch sowie CPU-, RAM-, Festplatten- (`/data` & `/`), Netzwerk- und Disk-I/O-Graphen.
- **PWA & 11 Sprachen**: Google-Sans-Typografie, Hell/Dunkel/System-Design, Logo/Icons/Manifest.

## Schnellstart (Docker)

```bash
docker compose up -d --build
```
