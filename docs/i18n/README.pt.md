<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Português)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <strong>Português</strong> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Monitor e painel de controle leve em Go para múltiplas contas de máquinas virtuais **instabox.sh**.

## Recursos

- **Binário único em Go** (`CGO_ENABLED=0`, interface web embutida com `embed.FS` e SQLite `modernc.org/sqlite`).
- **Cadastro inicial de administrador**: bloqueado após criar o primeiro usuário admin (`scrypt`, cookie `HttpOnly`, proteção CSRF).
- **Múltiplas contas com alias**: associe várias API Keys `ibk_...` criptografadas com AES-256-GCM.
- **Grade / Cartões e indicador de status por cor**: visualização rápida e filtrável.
- **4 botões apenas com ícones SVG + tooltip**: Acordar, Pausar, Iniciar e Parar imediatamente via API oficial `instabox.sh`.
- **Relógio em tempo real e gráficos estilo Beszel**: cronômetro baseado em `runStartedAt`, uso mensal e gráficos de CPU, RAM, Disco (`/data` e `/`), Rede e E/S de disco.
- **PWA e 11 idiomas**: fonte Google Sans, tema Claro/Escuro/Sistema, ícones e manifesto web.

## Início rápido (Docker)

```bash
docker compose up -d --build
```
