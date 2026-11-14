<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Português)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <strong>Português</strong> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Monitor e painel leve em Go para gerenciar múltiplas contas **instabox.sh**, manter VMs ativas 24/7 e orquestrar tudo via MCP remoto.

## Recursos

- **Binário único em Go** (`CGO_ENABLED=0`, interface web embutida com `embed.FS` e SQLite `modernc.org/sqlite`).
- **Cadastro inicial de administrador**: bloqueado após criar o primeiro usuário admin (`scrypt`, cookie `HttpOnly`, proteção CSRF e Fetch Metadata).
- **Múltiplas contas e Adicionar Máquina em 2 abas**: associe várias API Keys `ibk_...` criptografadas com AES-256-GCM ou crie novas VMs escolhendo template/snapshot, especificações e limites com visualização de cota em tempo real.
- **Renomeação inline, exclusão confirmada e modos Grade/Cartões**: pontos de status coloridos (tratando corretamente `runtime.ready=false` após retomada), cartões alinhados em telas largas, edição de nome no próprio cartão e confirmação obrigatória ao excluir.
- **Modo manter ativo por VM (`keepalive`)**: renovação automática de lease `ibmon-keepalive` (3600 s), reativação após pausa por ociosidade, reinício preventivo antes do limite de 8 h / `maxRunMinutes` e respeito às paradas manuais.
- **4 botões apenas com ícones SVG + tooltip**: Acordar, Pausar, Iniciar e Parar imediatamente via API oficial `instabox.sh`.
- **Relógio em tempo real e gráficos estilo Beszel**: cronômetro `runStartedAt`, uso mensal e gráficos de CPU, RAM, Disco (`/` e `/data`), Rede e E/S de disco (coleta a cada 30 s, backfill de 24 h, retenção 7 dias).
- **Chaves API de serviço (`ibmon_*`) e MCP Remoto (`/mcp`)**: gere e revogue tokens Bearer com hash SHA-256 em **Menu → Chaves API e MCP** para que o Hermes Agent ou scripts controlem todas as VMs (12 ferramentas MCP + API REST) sem expor as chaves `ibk_*`. A exclusão via API/MCP exige token de confirmação de uso único (5 min).
- **PWA e 11 idiomas**: fonte Google Sans, tema Claro/Escuro/Sistema, ícones e manifesto web.

## Início rápido (Docker)

```bash
docker compose up -d --build
```
