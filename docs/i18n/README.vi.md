<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Tiếng Việt)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <strong>Tiếng Việt</strong> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Bảng điều khiển và giám sát các máy ảo đám mây **instabox.sh** đa tài khoản, viết bằng **Go thuần nhẹ** (`CGO_ENABLED=0`) kèm giao diện web nhúng và cơ sở dữ liệu SQLite nhúng.

## Tính năng chính

- **Kiến trúc Go gọn nhẹ:** Một file thực thi duy nhất chứa toàn bộ tài nguyên tĩnh (`embed.FS`), dữ liệu lưu tại SQLite nội bộ (`modernc.org/sqlite`).
- **Tạo tài khoản Admin lần đầu:** Tự khóa đăng ký sau khi tạo admin duy nhất (`scrypt`, cookie `HttpOnly`, bảo vệ CSRF & Fetch Metadata).
- **Quản lý nhiều tài khoản instabox.sh:** Đặt tên gợi nhớ (alias) và gắn API Key (`ibk_...`). Key được mã hóa AES-256-GCM trên server và không bao giờ trả nguyên bản về trình duyệt.
- **Hiển thị Dạng lưới / Dạng thẻ & Point Checker:** Chấm màu trạng thái trực quan (`running`, `booting`, `idle`, `suspended`, `stopped`, `error/cooldown`).
- **4 nút thao tác SVG kèm chú thích khi rê chuột:** Đánh thức (`wake`/`resume`), Tạm dừng (`suspend`), Bật (`start`) và Tắt máy ngay lập tức (`stop`, không hỏi xác nhận) gọi qua API chính thức của `instabox.sh`.
- **Đồng hồ đếm giờ chạy thực tế & Biểu đồ kiểu Beszel:** Đồng hồ giây đếm từ `runStartedAt` đồng bộ lệch giờ server, tổng giờ chạy trong tháng và biểu đồ tương tác CPU, RAM, Disk (`/data` & `/`), Network, Disk I/O (thu thập ngầm mỗi 30 giây, lưu lịch sử mặc định 7 ngày).
- **Modern Web & PWA:** Font Google Sans, giao diện Sáng/Tối/Hệ thống, logo/icon/manifest để cài đặt thành ứng dụng, hỗ trợ 11 ngôn ngữ.

## Khởi chạy nhanh với Docker

```bash
docker compose up -d --build
```
