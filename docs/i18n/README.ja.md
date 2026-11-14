<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (日本語)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <strong>日本語</strong> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

複数アカウントの **instabox.sh** クラウド VM を監視・常時稼働維持・リモート MCP 連携でオーケストレーションできる軽量 Go 製コンソールです。

## 主な機能

- **軽量な単一 Go バイナリ**：`CGO_ENABLED=0` の純粋 Go 実装、`embed.FS` と組み込み SQLite (`modernc.org/sqlite`) を採用。
- **初回管理者セットアップ**：初回アクセス時に管理者アカウントを 1 つ作成して登録をロック（`scrypt`、`HttpOnly` Cookie、CSRF・Fetch Metadata 保護）。
- **マルチアカウント管理＆2タブのマシン追加**：複数の `ibk_...` キー（AES-256-GCM 暗号化保存）の登録に加え、空きクォータを確認しながらテンプレート／スナップショットから新規 VM を直接作成可能。
- **インライン名前変更・確認付き削除・整列カードビュー**：状態に応じたカラーインジケーター（スナップショット復帰後の `runtime.ready=false` にも適切に対応）、ワンクリック名前変更、削除確認ダイアログを搭載。
- **対話型コンソールターミナル（`xterm.js` + WebSocket）**：稼働中・アイドル休止中の各 VM に対してブラウザから直接 PTY（`xterm-256color`）ターミナルを開き、永続セッション名切替（`main`/`dev`/`logs`）、自動リサイズ、全画面、シェル再起動、モバイル用クイックキーを利用可能（Go バックエンドで安全に中継）。
- **VM ごとのスリープ防止＆自動サイクル (`keepalive`)**：`ibmon-keepalive` リース（3600 秒）を自動更新し、アイドル休止からの自動復帰や 8 時間／`maxRunMinutes` 上限前の再起動を実施（手動停止は尊重）。
- **4 つの SVG アイコン操作ボタン**：ホバー時にツールチップ表示。ウェイク（`wake`/`resume`）、一時停止（`suspend`）、起動（`start`）、即時停止（`stop`）に対応。
- **リアルタイム稼働タイマー＆Beszel 風チャート**：`runStartedAt` 連動タイマー、月間稼働時間、CPU・RAM・ディスク（`/`・`/data`）・ネットワーク・ディスク I/O グラフ（30秒間隔収集・24時間補完・7日間保持）。
- **サービス API キー (`ibmon_*`) ＆リモート MCP (`/mcp`)**：**メニュー → APIキー & MCP** から SHA-256 ハッシュ化 Bearer トークンを発行・無効化し、Hermes Agent やスクリプトから保存済み VM を操作可能（元の `ibk_*` キーは非公開）。API/MCP 経由の VM 削除は 5 分間有効の単発確認トークンによる 2 段階確認を必須化。
- **Modern Web & PWA**：Google Sans フォント、ライト/ダーク/システム切替、SVG/PNG アイコン、マニフェスト、11言語 UI 対応。

## クイックスタート (Docker)

```bash
docker compose up -d --build
```
