<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (한국어)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <strong>한국어</strong> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

멀티 계정 **instabox.sh** 클라우드 VM을 모니터링하고 24시간 절전 방지 및 원격 MCP 오케스트레이션까지 지원하는 경량 Go 웹 콘솔입니다.

## 주요 기능

- **경량 단일 Go 바이너리**: `CGO_ENABLED=0` 순수 Go + 내장 웹 자산(`embed.FS`) + SQLite(`modernc.org/sqlite`).
- **최초 관리자 계정 설정**: 첫 실행 시 관리자 계정을 한 번만 생성하도록 잠금(`scrypt`, `HttpOnly` 쿠키, CSRF 및 Fetch Metadata 방어).
- **다중 계정 관리 및 2탭 머신 추가**: 여러 `instabox.sh` API 키(`ibk_...`, AES-256-GCM 암호화 저장)를 연결하거나, 사용 가능한 할당량을 확인해 템플릿/스냅샷으로 새 VM을 즉시 생성합니다.
- **인라인 이름 변경, 확인 후 삭제 및 정렬된 카드/그리드 보기**: 상태별 색상 표시기(스냅샷 재개 후 `runtime.ready=false` 상태도 정확히 처리), 카드 내 이름 변경 및 삭제 확인 모달을 제공합니다.
- **대화형 콘솔 터미널 (`xterm.js` + WebSocket)**: 실행 중이거나 절전 상태인 각 VM에 브라우저에서 바로 PTY(`xterm-256color`) 터미널 창을 열어 영구 세션(`main`, `dev`, `logs` 등), 자동 크기 맞춤, 전체 화면, 셸 재시작, 모바일 퀵키 바를 사용할 수 있습니다.
- **VM별 절전 방지 및 자동 재시작(`keepalive`)**: `ibmon-keepalive` 임대를 3600초 주기로 갱신하고 유휴 정지 시 자동 기동하며 8시간/`maxRunMinutes` 한도 전 선제적 재시작을 수행합니다(수동 중지는 유지).
- **SVG 아이콘 전용 4개 제어 버튼**: 호버 시 툴팁 표시 — 깨우기(`wake`/`resume`), 일시중지(`suspend`), 시작(`start`), 즉시 중지(`stop`).
- **실시간 가동 시계 & Beszel 스타일 차트**: `runStartedAt` 기반 실시간 타이머, 월간 누적 시간, CPU/RAM/디스크(`/`, `/data`), 네트워크, 디스크 I/O 그래프(30초 수집, 24시간 백필, 7일 보관).
- **서비스 API 키(`ibmon_*`) 및 원격 MCP(`/mcp`)**: **메뉴 → API 키 & MCP**에서 SHA-256 해시 기반 Bearer 토큰을 발급·폐기하여 Hermes Agent 또는 스크립트가 원본 `ibk_*` 키 노출 없이 모든 VM을 제어할 수 있습니다(12개 MCP 도구 + REST API 제공, 머신 삭제는 5분 유효 1회용 확인 토큰으로 2단계 확인 필수).
- **Modern Web & PWA**: Google Sans 폰트, 라이트/다크/시스템 테마, 로고/아이콘/매니페스트 및 11개 언어 지원.

## 빠른 시작 (Docker)

```bash
docker compose up -d --build
```
