<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Русский)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <a href="README.vi.md">Tiếng Việt</a> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <strong>Русский</strong>
</p>

Лёгкая панель мониторинга и управления облачными ВМ **instabox.sh** для нескольких аккаунтов на базе Go.

## Возможности

- **Один лёгкий бинарный файл Go** (`CGO_ENABLED=0`, встроенный веб-интерфейс через `embed.FS`, встроенная SQLite `modernc.org/sqlite`).
- **Первичная настройка администратора**: регистрация блокируется после создания первого администратора (`scrypt`, cookie `HttpOnly`, защита CSRF).
- **Несколько аккаунтов с псевдонимами**: привязка API-ключей `ibk_...` с шифрованием AES-256-GCM.
- **Сетка / Карточки и цветовой индикатор статуса**: мгновенная визуализация состояния каждой ВМ.
- **4 SVG-кнопки управления с подсказками при наведении**: разбудить, приостановить, запустить и немедленно остановить через официальный API `instabox.sh`.
- **Таймер работы в реальном времени и графики в стиле Beszel**: отсчёт по `runStartedAt`, статистика за месяц, графики ЦП, ОЗУ, Диска (`/data` и `/`), Сети и Диска I/O (сбор каждые 30 сек, хранение 7 дней).
- **PWA и 11 языков**: шрифт Google Sans, светлая/тёмная/системная тема, иконки и веб-манифест.

## Быстрый запуск (Docker)

```bash
docker compose up -d --build
```
