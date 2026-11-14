<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Español)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <strong>Español</strong> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Consola ligera en Go y Web moderna para supervisar, mantener activas 24/7 y orquestar mediante MCP remoto máquinas virtuales **instabox.sh** multicuenta.

## Características

- **Binario único y ligero en Go** (`CGO_ENABLED=0`, UI integrada con `embed.FS` y SQLite `modernc.org/sqlite`).
- **Configuración inicial de administrador**: bloqueo tras el primer registro (`scrypt`, cookie `HttpOnly`, protección CSRF y Fetch Metadata).
- **Múltiples cuentas y modal Añadir máquina en 2 pestañas**: vincula varias API Keys `ibk_...` cifradas con AES-256-GCM o crea nuevas VM directamente consultando cuotas, plantillas/snapshots, especificaciones y política de inactividad.
- **Renombrado directo, eliminación confirmada y vistas Cuadrícula/Tarjetas**: indicadores de estado en vivo (manejando correctamente `runtime.ready=false` tras reanudación), tarjetas alineadas en escritorio, edición en línea del nombre y diálogo de confirmación antes de eliminar.
- **Antisuspensión por VM (`keepalive`)**: renovación automática del lease `ibmon-keepalive` (3600 s), reactivación tras pausa por inactividad, reinicio preventivo antes del límite de 8 h / `maxRunMinutes` y respeto por las paradas manuales.
- **4 botones SVG con descripción al pasar el cursor**: Despertar, Pausar, Iniciar y Detener inmediatamente mediante la API oficial de `instabox.sh`.
- **Reloj en vivo y gráficos estilo Beszel**: contador desde `runStartedAt`, uso mensual y gráficos interactivos de CPU, RAM, Disco (`/` y `/data`), Red y E/S de disco (sondeo cada 30 s, relleno de 24 h, retención 7 días).
- **Claves API de servicio (`ibmon_*`) y MCP remoto (`/mcp`)**: crea y revoca tokens Bearer con hash SHA-256 en **Menú → Claves API y MCP** para que Hermes Agent o tus scripts controlen las VM (12 herramientas MCP + API REST de orquestación) sin exponer las claves `ibk_*`. La eliminación mediante API/MCP exige confirmación explícita con token de un solo uso (5 min).
- **PWA y 11 idiomas**: tipografía Google Sans, tema Claro/Oscuro/Sistema, iconos y manifiesto web.

## Inicio rápido (Docker)

```bash
docker compose up -d --build
```
