<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Français)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <strong>Français</strong> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Console légère en Go et Web moderne pour superviser, maintenir éveillées et orchestrer via MCP distant vos machines virtuelles **instabox.sh** multi-comptes.

## Fonctionnalités

- **Binaire Go unique et léger** (`CGO_ENABLED=0`, `embed.FS`, SQLite pur Go `modernc.org/sqlite`).
- **Configuration admin initiale** : création unique de l’administrateur au premier lancement (`scrypt`, cookie `HttpOnly`, protection CSRF et Fetch Metadata).
- **Multi-comptes & Ajout de machine en 2 onglets** : associez plusieurs comptes (`ibk_...`, chiffrés en AES-256-GCM) ou créez de nouvelles VM directement selon la capacité et les quotas disponibles (modèle/snapshot, vCPU/RAM, mise en veille, durée maximale).
- **Renommage en ligne, suppression confirmée & vues Grille/Cartes** : pastilles d’état en temps réel (gérant correctement `runtime.ready=false` après reprise de snapshot), cartes alignées sur écran large, renommage direct et confirmation obligatoire avant suppression.
- **Console Terminal interactive (`xterm.js` via WebSocket)** : ouvrez un terminal PTY (`xterm-256color`) directement dans le navigateur pour chaque VM active ou en veille, avec sessions persistantes nommées (`main`, `dev`, `logs`…), redimensionnement automatique, plein écran, redémarrage du shell et barre de raccourcis mobile via proxy Go sécurisé.
- **Maintien éveillé par VM (`keepalive`)** : renouvellement automatique du bail `ibmon-keepalive` (3600 s), réveil après veille automatique, redémarrage préventif avant la limite 8 h / `maxRunMinutes` et respect strict des arrêts manuels.
- **4 boutons SVG avec infobulle au survol** : Réveiller, Mettre en pause, Démarrer et Arrêter immédiatement via l’API officielle `instabox.sh`.
- **Chronomètre en direct & graphiques façon Beszel** : compteur `runStartedAt`, cumul mensuel et graphiques CPU, RAM, Disque (`/` et `/data`), Réseau et E/S Disque (collecte 30 s, rattrapage 24 h, rétention 7 jours).
- **Clés API de service (`ibmon_*`) & MCP distant (`/mcp`)** : générez et révoquez des jetons Bearer hachés SHA-256 depuis **Menu → Clés API & MCP** pour permettre à Hermes Agent ou vos scripts de piloter toutes vos VM (12 outils MCP et endpoints REST d’orchestration) sans jamais exposer les clés `ibk_*` d’origine. La suppression d’une VM via API/MCP exige un jeton de confirmation à usage unique (5 min) après accord de l’utilisateur.
- **PWA & 11 langues** : typographie Google Sans, thème Clair/Sombre/Système, manifeste et icônes SVG/PNG.

## Démarrage rapide (Docker)

```bash
docker compose up -d --build
```
