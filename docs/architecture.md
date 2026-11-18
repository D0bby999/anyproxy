# Kiến trúc & Phát triển AnyProxy

Tài liệu này mô tả các quyết định thiết kế, ranh giới module và quy trình build/test của **AnyProxy**. Hành vi chi tiết nằm trực tiếp trong mã nguồn và test của từng package.

---

## 1. Các quyết định thiết kế cốt lõi (WHY)

| Chủ đề | Lựa chọn hiện tại | Lý do & Ràng buộc |
|---|---|---|
| **Cơ chế chặn/định tuyến lưu lượng** | Kernel/System Extension (`NEDriver` trên macOS, `WinDivertDriver` trên Windows) + Fallback `SysProxyDriver` & Local Inbound Proxy (`127.0.0.1:10800`) | Bản release chính thức dùng `NETransparentProxyProvider` + `NEDNSProxyProvider` (macOS) và `WinDivert` + `ProxyBridgeCore.dll` (Windows) để bắt toàn bộ TCP/UDP theo từng ứng dụng. Khi chạy bản dev không có provisioning profile/admin hoặc đặt `ANYPROXY_DRIVER=sysproxy`, tự động fallback về OS System Proxy kết hợp cổng local `127.0.0.1:10800`. |
| **Dual-mode Entrypoint** | Một binary cho cả GUI (Wails v2) và CLI | [`cli.ShouldRunCLI`](../internal/cli/cli.go) định tuyến dựa trên cờ dòng lệnh; cả hai chế độ dùng chung một [`service.Service`](../internal/service/service.go) để không lệch logic rule/proxy. |
| **Lưu trữ trạng thái** | Atomic JSON (`state.json`) thay cho SQLite | Dữ liệu cấu hình (profiles, proxies, rules, settings) nhỏ, đọc/ghi toàn phần qua ghi file tạm + rename trong [`internal/store`](../internal/store/store.go), bỏ hoàn toàn CGO SQLite và đơn giản hóa cross-compile. |
| **Lưu trữ mật khẩu** | AES-256-GCM vault (`secrets.vault`) + OS Keychain/Credential Manager | Mật khẩu proxy không bao giờ nằm trong `state.json`. Toàn bộ bí mật nằm trong một file mã hóa ở [`internal/secrets`](../internal/secrets/vault.go), chỉ khóa 32-byte lưu ở Keychain (macOS) / Credential Manager (Windows) để tránh nhiều prompt cấp quyền. |
| **Đánh giá Rule** | Pre-compiled matcher, first-match-wins | [`rules.Engine`](../internal/rules/matcher.go) biên dịch trước các pattern khi sync profile; nếu rule `PROXY` trỏ tới proxy không tồn tại sẽ fail-closed thành `BLOCK`, còn loopback/multicast/stub-DNS (`:53`) bị ép về `DIRECT` qua [`rules.ShouldBypassProxy`](../internal/rules/bypass.go). |

---

## 2. Bản đồ Module (WHERE)

| Thành phần | Chủ sở hữu | Vai trò |
|---|---|---|
| **Entrypoint & Window Host** | [main.go](../main.go), [app.go](../app.go), [app_bindings.go](../app_bindings.go) | Khởi tạo Wails GUI hoặc chuyển sang CLI; giữ vòng đời cửa sổ (Full/Compact), khôi phục vị trí, và expose bindings cho frontend. |
| **Coordinator trung tâm** | [internal/service](../internal/service/service.go) | Điều phối `store`, `vault`, `inbound`, `driver`, `flows`, `dnsmap`, và phát sự kiện (`profile:changed`, `engine:status`). |
| **Local Inbound & PID Resolver** | [internal/inbound/server.go](../internal/inbound/server.go), [proc.go](../internal/inbound/proc.go) | Lắng nghe SOCKS5/HTTP trên `127.0.0.1:10800`; ánh xạ cổng nguồn sang PID/tên tiến trình qua snapshot `netstat` + `ps`/`tasklist`. |
| **OS Interception Drivers** | [internal/driver/darwin_ne.go](../internal/driver/darwin_ne.go), [windows_windivert.go](../internal/driver/windows_windivert.go), [sysproxy.go](../internal/driver/sysproxy.go), [macos/extension](../macos/extension), [windows/core](../windows/core) | Điều khiển System Extension (macOS), WinDivert/ProxyBridgeCore (Windows), hoặc fallback sang System Proxy (`networksetup` / `reg`) và tự tắt cờ `sysproxy_on` khi khởi động lại sau crash. |
| **Upstream Relay & Checker** | [internal/netproxy/upstream.go](../internal/netproxy/upstream.go), [checker.go](../internal/netproxy/checker.go) | Chuyển tiếp TCP tới SOCKS5/HTTP upstream và chạy 5-step Proxy Checker (bao gồm SOCKS5 UDP Associate probe). |
| **Rule Engine & Profile Schema** | [internal/rules/matcher.go](../internal/rules/matcher.go), [internal/pbprofile](../internal/pbprofile/pbprofile.go) | Định dạng `.pbprofile` (tương thích ProxyBridge PascalCase/camelCase) và bộ khớp quy tắc theo Process/Host/CIDR/Port/Protocol. |
| **Persistence & Secrets** | [internal/paths](../internal/paths/paths.go), [internal/store](../internal/store/store.go), [internal/secrets](../internal/secrets/vault.go) | Quản lý thư mục dữ liệu owner-only (`0700`/`0600`), hỗ trợ tương thích ngược thư mục `AnyProxyXa`, lưu JSON và vault mã hóa. |
| **Telemetry & Update** | [internal/flows](../internal/flows/tracker.go), [internal/dnsmap](../internal/dnsmap/store.go), [internal/update](../internal/update/update.go) | Theo dõi live flows, ring buffer logs, cache DNS `/etc/hosts`, kiểm tra phiên bản mới từ `latest.json` (`ANYPROXY_UPDATE_URL`). |
| **CLI Runner** | [internal/cli](../internal/cli/cli.go) | Xử lý các cờ headless và stream log ra `stdout`. |
| **Frontend (React + TypeScript)** | [frontend/src/App.tsx](../frontend/src/App.tsx), [api.ts](../frontend/src/api.ts), [preview.html](../frontend/preview.html) | Giao diện Wails (Connections, Activity, Proxies, Rules, Settings, Compact View) và chế độ preview độc lập trên trình duyệt với mock bindings. |

---

## 3. Build, Test & Kiểm tra

Mọi lệnh build, test và lint chính thức được định nghĩa trong [Makefile](../Makefile) và cấu hình đóng gói nằm ở [wails.json](../wails.json).

### Yêu cầu môi trường
- **Go**: phiên bản theo [go.mod](../go.mod) (`1.26.0`).
- **Node.js & npm**: dùng cho bundle frontend trong `frontend/`.
- **Wails CLI v2**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.

### Các lệnh thường dùng

```bash
# Chạy chế độ dev (hot reload Wails + Vite)
make dev

# Chạy toàn bộ unit tests (Go race detector + TypeScript typecheck + Vitest)
make test

# Kiểm tra format và go vet (bao gồm cross-check cho Windows)
make lint

# Build app bundle cho macOS (arm64) kèm codesign
make build

# Build bộ cài Windows (amd64 + NSIS)
make build-windows
```

### Phát triển UI không cần Wails shell
Mở dev server của Vite trong `frontend/` và truy cập `preview.html` để kiểm tra giao diện trực tiếp trên trình duyệt với dữ liệu giả lập (mock Wails bindings).

### Cập nhật icon ứng dụng
Icon nguồn nằm tại [build/appicon.svg](../build/appicon.svg). Khi thay đổi icon, render lại `build/appicon.png` (`1024×1024`) bằng `rsvg-convert` trước khi chạy `make build`.
