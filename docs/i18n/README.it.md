<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Italiano)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <strong>Italiano</strong> · <a href="README.ru.md">Русский</a>
</p>

Console leggera in Go e Web moderno per il monitoraggio e controllo multi-account delle VM **instabox.sh**.

## Funzionalità

- **Binario Go leggero e autonomo** (`CGO_ENABLED=0`, `embed.FS`, SQLite in pure Go `modernc.org/sqlite`).
- **Configurazione admin al primo accesso**: creazione bloccata dopo il primo amministratore (`scrypt`, cookie `HttpOnly`, protezione CSRF).
- **Gestione multi-account con alias**: associa più API Key `ibk_...` cifrate con AES-256-GCM.
- **Vista Griglia / Schede e indicatore di stato**: punto colorato in tempo reale per ogni VM.
- **4 pulsanti SVG con tooltip al passaggio del mouse**: Risveglia, Sospendi, Avvia e Arresta immediatamente tramite API ufficiale `instabox.sh`.
- **Orologio in tempo reale & grafici stile Beszel**: timer basato su `runStartedAt`, ore mensili e grafici CPU, RAM, Disco (`/data` e `/`), Rete e I/O disco.
- **PWA & 11 lingue**: font Google Sans, tema Chiaro/Scuro/Sistema, icone e manifest web.

## Avvio rapido (Docker)

```bash
docker compose up -d --build
```
