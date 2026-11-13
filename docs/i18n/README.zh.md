<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (中文)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <strong>中文</strong> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

面向多账户 **instabox.sh** 云端沙箱的轻量级 Go 监控与控制面板，采用单二进制部署与内嵌现代 Web 界面。

## 核心特性

- **轻量 Go 单二进制架构**：纯 Go (`CGO_ENABLED=0`)，通过 `embed.FS` 内嵌前端资源并使用纯 Go SQLite (`modernc.org/sqlite`) 存储。
- **首次运行创建管理员**：首次访问设置唯一的管理员账号（`scrypt` 哈希、`HttpOnly` 会话 Cookie、CSRF 与 Fetch Metadata 防护）。
- **多账户别名管理**：支持添加多个账户别名并绑定 `instabox.sh` API Key（`ibk_...`），服务端采用 AES-256-GCM 加密存储。
- **网格 / 卡片视图与状态点检查器**：直观颜色状态指示点，支持搜索与账户/状态过滤。
- **4 个纯 SVG 图标操作按钮**：悬停/聚焦显示说明提示，支持唤醒（`wake`/`resume`）、暂停（`suspend`）、启动（`start`）与一键立即停止（`stop`）。
- **实时运行时钟与类 Beszel 资源图表**：基于 `runStartedAt` 的逐秒计时器、本月累计时长，以及 CPU、内存、磁盘（`/data` 与 `/`）、网络和磁盘 I/O 交互式图表（每 30 秒后台轮询，默认保留 7 天）。
- **Modern Web & PWA**：采用 Google Sans 字体、深色/浅色/系统主题切换、SVG/PNG 图标与 `manifest.webmanifest`，内置 11 种界面语言。

## Docker 快速启动

```bash
docker compose up -d --build
```
