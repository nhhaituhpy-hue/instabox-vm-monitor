<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (한국어)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <strong>한국어</strong> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

멀티 계정 **instabox.sh** 클라우드 VM을 모니터링하고 제어하는 경량 Go 기반 웹 콘솔입니다.

## 주요 기능

- **경량 단일 Go 바이너리**: `CGO_ENABLED=0` 순수 Go + 내장 웹 자산(`embed.FS`) + SQLite(`modernc.org/sqlite`).
- **최초 관리자 계정 설정**: 첫 실행 시 관리자 계정을 한 번만 생성하도록 잠금(`scrypt`, `HttpOnly` 쿠키, CSRF 방어).
- **다중 계정 별칭 관리**: 여러 `instabox.sh` 계정에 별칭과 API 키(`ibk_...`)를 등록하며 서버에 AES-256-GCM으로 암호화 저장.
- **그리드/카드 보기 및 상태 포인트 체커**: VM 상태별 색상 표시기 제공.
- **SVG 아이콘 전용 4개 제어 버튼**: 마우스 호버 시 툴팁 표시 — 깨우기(`wake`/`resume`), 일시중지(`suspend`), 시작(`start`), 즉시 중지(`stop`).
- **실시간 가동 시계 & Beszel 스타일 차트**: `runStartedAt` 기반 실시간 타이머, 월간 누적 사용 시간, CPU/RAM/디스크(`/data`, `/`), 네트워크, 디스크 I/O 그래프(30초 주기 수집, 7일 보관).
- **Modern Web & PWA**: Google Sans 폰트, 라이트/다크/시스템 테마, 로고/아이콘/매니페스트 및 11개 언어 지원.

## 빠른 시작 (Docker)

```bash
docker compose up -d --build
```
