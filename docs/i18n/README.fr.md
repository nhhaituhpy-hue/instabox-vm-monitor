<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Français)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <strong>Français</strong> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Console légère en Go et Web moderne pour superviser et piloter vos machines virtuelles **instabox.sh** multi-comptes.

## Fonctionnalités

- **Binaire Go unique et léger** (`CGO_ENABLED=0`, `embed.FS`, SQLite pur Go `modernc.org/sqlite`).
- **Configuration admin initiale** : création unique de l’administrateur au premier lancement (`scrypt`, cookie `HttpOnly`, protection CSRF).
- **Gestion multi-comptes** : ajoutez plusieurs comptes avec alias et clé API (`ibk_...`) chiffrée en AES-256-GCM.
- **Vues Grille / Cartes & indicateur d’état** : pastille de couleur en temps réel par état VM.
- **4 boutons SVG avec infobulle au survol** : Réveiller, Mettre en pause, Démarrer et Arrêter immédiatement via l’API officielle `instabox.sh`.
- **Chronomètre en direct & graphiques façon Beszel** : compteur d’exécution `runStartedAt`, cumul mensuel et graphiques CPU, RAM, Disque (`/data` et `/`), Réseau et E/S Disque (collecte toutes les 30 s, rétention 7 jours).
- **PWA & 11 langues** : typographie Google Sans, thème Clair/Sombre/Système, manifeste et icônes SVG/PNG.

## Démarrage rapide (Docker)

```bash
docker compose up -d --build
```
