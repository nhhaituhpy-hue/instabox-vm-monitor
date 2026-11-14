<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (中文)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <strong>中文</strong> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

面向多账户 **instabox.sh** 云端沙箱的轻量级 Go 监控、7×24 防休眠保活与远程 MCP 编排控制面板，采用单二进制部署与内嵌现代 Web 界面。

## 核心特性

- **轻量 Go 单二进制架构**：纯 Go (`CGO_ENABLED=0`)，通过 `embed.FS` 内嵌前端资源并使用纯 Go SQLite (`modernc.org/sqlite`) 存储。
- **首次运行创建管理员**：首次访问设置唯一的管理员账号（`scrypt` 哈希、`HttpOnly` 会话 Cookie、CSRF 与 Fetch Metadata 防护）。
- **多账户管理与双标签“添加虚拟机”**：支持绑定多个 `instabox.sh` 账户 API Key（`ibk_...`，服务端 AES-256-GCM 加密存储），也可直接在已有配额的账户上选择模板/快照、规格、自动休眠和最长运行时长创建新虚拟机。
- **内联重命名、安全删除与对齐的网格/卡片视图**：直观的状态色点（正确处理唤醒或无桌面虚拟机在 `runtime.ready=false` 时的运行态）、支持卡片内重命名、带二次确认弹窗的删除，以及宽屏对齐的卡片布局。
- **内置交互式控制台终端（基于 `xterm.js` 与 WebSocket）**：直接在浏览器中为运行中或休眠唤醒的虚拟机打开实时 PTY（`xterm-256color`）终端窗口，支持持久命名会话（`main`、`dev`、`logs` 等）、自适应缩放、全屏、重启 Shell 与移动端快捷按键栏，并由 Go 后端安全代理转发。
- **单机防休眠与自动循环重启（Keep-awake）**：可为每台虚拟机单独开启保活，自动续期 `ibmon-keepalive` 租约（3600 秒）、在冷却结束后唤醒空闲暂停的虚拟机，并在触及 8 小时 / `maxRunMinutes` 上限前自动重启，同时尊重手动停止操作。
- **4 个纯 SVG 图标操作按钮**：悬停/聚焦显示提示，支持唤醒（`wake`/`resume`）、暂停（`suspend`）、启动（`start`）与一键立即停止（`stop`）。
- **实时运行时钟与类 Beszel 资源图表**：基于 `runStartedAt` 的逐秒计时器、本月累计时长，以及 CPU、内存、磁盘（`/` 与 `/data`）、网络和磁盘 I/O 交互式图表（30 秒轮询、24 小时回溯、默认保留 7 天）。
- **服务 API 密钥（`ibmon_*`）与远程 MCP（`/mcp`）**：在**菜单 → API 密钥与 MCP** 中生成与吊销经过 SHA-256 哈希的 Bearer 令牌（仅创建时显示一次），使 Hermes Agent 或脚本可通过 12 个 MCP 工具与 REST 编排接口全权管理所有已保存虚拟机，且绝不暴露上游 `ibk_*` 密钥；通过 API 密钥或 MCP 删除虚拟机时强制要求一次性的 5 分钟作用域确认令牌二次确认。
- **Modern Web & PWA**：采用 Google Sans 字体、深色/浅色/系统主题切换、SVG/PNG 图标与 `manifest.webmanifest`，内置 11 种界面语言。

## 连接 Hermes Agent（远程 MCP）

1. 登录 Web 控制台，在**菜单 → API 密钥与 MCP** 中创建密钥（`ibmon_...` 仅展示一次）。
2. 将 Token 存入 `~/.hermes/.env`（`MCP_INSTABOX_API_KEY`），并在 `~/.hermes/config.yaml` 添加：

```yaml
mcp_servers:
  instabox:
    url: "https://your-monitor-host/mcp"
    headers:
      Authorization: "Bearer ${MCP_INSTABOX_API_KEY}"
    timeout: 620
    connect_timeout: 60
    enabled: true
```

## Docker 快速启动

```bash
docker compose up -d --build
```
