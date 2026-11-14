<p align="center">
  <img src="../../internal/web/static/assets/logo.svg" alt="InstaBox VM Monitor Logo" width="84" height="84" />
</p>

<h1 align="center">InstaBox VM Monitor (Tiếng Việt)</h1>

<p align="center">
  <a href="../../README.md">English</a> · <strong>Tiếng Việt</strong> · <a href="README.zh.md">中文</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.es.md">Español</a> · <a href="README.pt.md">Português</a> · <a href="README.de.md">Deutsch</a> · <a href="README.it.md">Italiano</a> · <a href="README.ru.md">Русский</a>
</p>

Bảng điều khiển, giám sát, chống ngủ tự động 24/7 và điều phối qua Remote MCP cho các máy ảo đám mây **instabox.sh** đa tài khoản, viết bằng **Go thuần nhẹ** (`CGO_ENABLED=0`) kèm giao diện web và SQLite nhúng.

## Tính năng chính

- **Kiến trúc Go gọn nhẹ:** Một file thực thi duy nhất chứa toàn bộ tài nguyên tĩnh (`embed.FS`), dữ liệu lưu tại SQLite nội bộ (`modernc.org/sqlite`).
- **Tạo tài khoản Admin lần đầu:** Tự khóa đăng ký sau khi tạo admin duy nhất (`scrypt`, cookie `HttpOnly`, bảo vệ CSRF & Fetch Metadata).
- **Quản lý nhiều tài khoản & Luồng Thêm máy 2 tab:** Gắn nhiều tài khoản `instabox.sh` (`ibk_...`, mã hóa AES-256-GCM trên server) hoặc tạo máy ảo mới ngay trên tài khoản còn hạn mức (chọn template/snapshot, cấu hình vCPU/RAM/disk, thời gian auto-idle và `maxRunMinutes`).
- **Đổi tên, Xóa VM an toàn & Giao diện Dạng lưới / Dạng thẻ:** Chấm màu trạng thái trực quan (phân biệt chính xác máy đang chạy kể cả khi `runtime.ready=false` sau resume), đổi tên trực tiếp tại thẻ máy, dialog xác nhận trước khi xóa VM, và thẻ ngang căn thẳng hàng trên màn hình rộng.
- **Cửa sổ Console Terminal trực tiếp (`xterm.js` qua WebSocket):** Mở terminal PTY (`xterm-256color`) ngay trên trình duyệt cho từng VM đang chạy/nghỉ (`GET /api/machines/{accountId}/{machineId}/terminal`), hỗ trợ phiên bền vững theo tên (`main`, `dev`, `logs`, ...), tự co giãn kích thước, toàn màn hình, khởi động lại shell và thanh phím nhanh cho điện thoại mà không lộ key `ibk_*`.
- **Chống ngủ & giữ chạy (`keepalive`):** Checkbox bật/tắt trên từng VM để tự gia hạn lease `ibmon-keepalive` (`3600s`), tự đánh thức khi bị auto-idle hết cooldown, tự power-cycle trước ngưỡng 8h/`maxRunMinutes`, khôi phục trạng thái restart nếu bị ngắt, và tôn trọng máy tắt thủ công.
- **4 nút thao tác SVG kèm chú thích khi rê chuột:** Đánh thức (`wake`/`resume`), Tạm dừng (`suspend`), Bật (`start`) và Tắt máy ngay lập tức (`stop`, không hỏi xác nhận) qua API chính thức của `instabox.sh`.
- **Đồng hồ đếm giờ chạy thực tế & Biểu đồ kiểu Beszel:** Đồng hồ giây đếm từ `runStartedAt` đồng bộ lệch giờ server, tổng giờ chạy trong tháng và biểu đồ tương tác CPU, RAM, Disk (`/data` & `/`), Network, Disk I/O (thu thập ngầm 30 giây, backfill 24h, lưu mặc định 7 ngày).
- **Service API Key (`ibmon_*`) & Remote MCP (`/mcp`):** Tạo và thu hồi token Bearer (băm SHA-256, chỉ hiển thị 1 lần lúc tạo) ngay trong **Menu → API Key & MCP** để Hermes Agent hoặc script điều phối toàn quyền mọi VM (`exec`, `files`, `ports`, `processes`, `logs`, `connection-info`, `snapshots`, `restore`, keepalive) mà không bao giờ lộ key `ibk_*` gốc. Riêng xóa machine qua API/MCP bắt buộc qua bước sinh token xác nhận 1 lần (hiệu lực 5 phút, gắn đúng máy) để hỏi lại xác nhận trước khi xóa.
- **Modern Web & PWA:** Font Google Sans, giao diện Sáng/Tối/Hệ thống, logo/icon/manifest để cài đặt thành ứng dụng, hỗ trợ 11 ngôn ngữ.

## Kết nối nhanh với Hermes Agent (Remote MCP)

1. Đăng nhập giao diện web → **Menu → API Key & MCP** → tạo API Key mới và sao chép token (`ibmon_...`).
2. Thêm vào `~/.hermes/config.yaml` (lưu token trong `~/.hermes/.env` với biến `MCP_INSTABOX_API_KEY`):

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

3. Chạy `/reload-mcp` (hoặc `hermes mcp test instabox`) để kích hoạt 12 tool MCP (`list_fleet`, `get_account_options`, `create_machine`, `control_machine`, `delete_machine`, `exec_command`, `read_files`, `write_file`, `manage_ports`, `inspect_machine`, `manage_snapshots`, `get_operation`).

## Khởi chạy nhanh với Docker

```bash
docker compose up -d --build
```
