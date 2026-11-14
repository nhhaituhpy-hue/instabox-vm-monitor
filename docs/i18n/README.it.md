<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Italiano)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <strong>Italiano</strong> · <a href="README.ru.md">Русский</a>
</p>

Console leggera in Go e Web moderno per il monitoraggio multi-account delle VM **instabox.sh**, gestione Keep-awake 24/7 e orchestrazione via MCP remoto.

## Funzionalità

- **Binario Go leggero e autonomo** (`CGO_ENABLED=0`, `embed.FS`, SQLite in pure Go `modernc.org/sqlite`).
- **Configurazione admin al primo accesso**: creazione bloccata dopo il primo amministratore (`scrypt`, cookie `HttpOnly`, protezione CSRF e Fetch Metadata).
- **Gestione multi-account & Aggiungi Macchina a 2 schede**: collega più chiavi API `ibk_...` cifrate con AES-256-GCM o crea nuove VM scegliendo account disponibili, template/snapshot, specifiche e limiti.
- **Rinomina inline, eliminazione confermata e vista Griglia/Schede**: indicatori di stato in tempo reale (gestione accurata di `runtime.ready=false` dopo resume), schede desktop allineate, modifica diretta del nome e finestra di conferma prima dell’eliminazione.
- **Console Terminale Interattiva (`xterm.js` su WebSocket)**: apri un terminale PTY (`xterm-256color`) nel browser per ogni VM attiva o in pausa, con sessioni persistenti con nome (`main`, `dev`, `logs`, ...), ridimensionamento automatico, schermo intero, riavvio shell e barra tasti rapidi per mobile.
- **Keep-awake per singola VM (`keepalive`)**: rinnovo automatico del lease `ibmon-keepalive` (3600 s), risveglio da sospensione automatica dopo cooldown, riavvio preventivo prima del limite 8 h / `maxRunMinutes` e rispetto degli arresti manuali.
- **4 pulsanti SVG con tooltip al passaggio del mouse**: Risveglia, Sospendi, Avvia e Arresta immediatamente tramite API ufficiale `instabox.sh`.
- **Orologio in tempo reale & grafici stile Beszel**: timer basato su `runStartedAt`, ore mensili e grafici CPU, RAM, Disco (`/` e `/data`), Rete e I/O disco (polling 30 s, backfill 24 h, retention 7 giorni).
- **API Key di servizio (`ibmon_*`) & MCP Remoto (`/mcp`)**: crea e revoca token Bearer con hash SHA-256 da **Menu → API Key & MCP** per consentire ad Hermes Agent o script di gestire tutte le VM (12 tool MCP e REST API) senza esporre le chiavi `ibk_*`. L’eliminazione di una VM via API/MCP richiede un token di conferma monouso valido 5 minuti.
- **PWA & 11 lingue**: font Google Sans, tema Chiaro/Scuro/Sistema, icone e manifest web.

## Avvio rapido (Docker)

```bash
docker compose up -d --build
```
