<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (日本語)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <strong>日本語</strong> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

複数アカウントの **instabox.sh** クラウド VM を監視・操作できる軽量 Go 製コンソールです。

## 主な機能

- **軽量な単一 Go バイナリ**：`CGO_ENABLED=0` の純粋 Go 実装、`embed.FS` と組み込み SQLite (`modernc.org/sqlite`) を採用。
- **初回管理者セットアップ**：初回アクセス時に管理者アカウントを 1 つ作成して登録をロック（`scrypt`、`HttpOnly` Cookie、CSRF 保護）。
- **マルチアカウント管理**：エイリアス名と `instabox.sh` API キー（`ibk_...`）を複数登録可能。AES-256-GCM で暗号化保存。
- **グリッド / カード表示とステータスドット**：稼働状態に応じた色分けポイントチェッカー。
- **4 つの SVG アイコン操作ボタン**：ホバー時にツールチップ表示。ウェイク（`wake`/`resume`）、一時停止（`suspend`）、起動（`start`）、即時停止（`stop`）に対応。
- **リアルタイム稼働タイマー＆Beszel 風チャート**：`runStartedAt` に連動した秒単位の稼働時計、月間稼働時間、CPU・RAM・ディスク（`/data`・`/`）・ネットワーク・ディスク I/O グラフ（30秒間隔収集・7日間保持）。
- **Modern Web & PWA**：Google Sans フォント、ライト/ダーク/システム切替、SVG/PNG アイコン、マニフェスト、11言語 UI 対応。

## クイックスタート (Docker)

```bash
docker compose up -d --build
```
