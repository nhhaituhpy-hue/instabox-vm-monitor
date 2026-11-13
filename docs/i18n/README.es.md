<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Español)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <strong>Español</strong> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Consola ligera en Go y Web moderna para supervisar y controlar máquinas virtuales **instabox.sh** multicuenta.

## Características

- **Binario único y ligero en Go** (`CGO_ENABLED=0`, UI integrada con `embed.FS` y SQLite `modernc.org/sqlite`).
- **Configuración inicial de administrador**: bloqueo tras el primer registro (`scrypt`, cookie `HttpOnly`, CSRF).
- **Múltiples cuentas con alias**: vincula varias API Keys `ibk_...` cifradas con AES-256-GCM.
- **Vista de cuadrícula / tarjetas y punto de estado**: indicador visual por color y filtros.
- **4 botones SVG con descripción al pasar el cursor**: Despertar, Pausar, Iniciar y Detener inmediatamente mediante la API oficial de `instabox.sh`.
- **Reloj en vivo y gráficos estilo Beszel**: contador desde `runStartedAt`, uso mensual y gráficos interactivos de CPU, RAM, Disco (`/data` y `/`), Red y E/S de disco.
- **PWA y 11 idiomas**: tipografía Google Sans, tema Claro/Oscuro/Sistema, iconos y manifiesto web.

## Inicio rápido (Docker)

```bash
docker compose up -d --build
```
