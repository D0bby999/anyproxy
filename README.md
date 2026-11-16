# AnyProxy

> **Free, open-source, Apple-notarized per-application proxy router & rule engine for macOS (Apple Silicon & Intel) and Windows.**
> An open-source Proxifier alternative built with **Go + Wails v2 + React/TypeScript**.
>
> - **Download (macOS `.dmg` & Windows `.exe`)**: [anytools.world/en/anyproxy](https://anytools.world/en/anyproxy) · [Tiếng Việt](https://anytools.world/vi/anyproxy)
> - **User Guide**: [English Guide](https://anytools.world/en/guides/anyproxy) · [Tài liệu tiếng Việt](docs/user-guide.md)
> - **Core Capabilities**: Unified `127.0.0.1:10800` SOCKS5 (with `UDP ASSOCIATE`) + HTTP `CONNECT` gateway · Top-down rules by process/PID (include & exclude), wildcard domain, IP/CIDR, port, and protocol · 5-Step Proxy Checker (TCP, Auth, GeoIP, 3x Latency/Jitter, UDP) · AES-256-GCM vault backed by macOS Keychain / Windows Credential Manager · Compact (`336×620`), Full (`1180×760`), and Headless CLI modes · `.pbprofile` XML import/export.

**AnyProxy** là công cụ điều hướng và giám sát lưu lượng mạng theo tiến trình (Per-application Proxy Router & Traffic Inspector) dành cho **macOS** và **Windows**, thuộc hệ sinh thái **AnyTools** (`https://anytools.world/vi/anyproxy`, cùng với `anyprofile`, `anybrowser`, `anymobile`).

Dự án được xây dựng bằng **Go + Wails v2 + React/TypeScript**, phát triển từ [InterceptSuite/ProxyBridge](https://github.com/InterceptSuite/ProxyBridge) với định dạng profile `.pbprofile` tương thích, lưu trữ trạng thái atomic JSON, mã hóa mật khẩu AES-256-GCM (khóa giữ trong OS Keychain / Credential Manager), và hỗ trợ cả **Desktop GUI** lẫn **Headless CLI** trong cùng một binary.

---

## Tài liệu

- **[Hướng dẫn sử dụng (User Guide)](docs/user-guide.md)** — Cách dùng GUI, tạo/sắp xếp Routing Rules, kiểm tra proxy 5 bước, Compact Mode, CLI flags, tích hợp `curl` / Burp Suite / AnyProfile, và xử lý sự cố.
- **[Kiến trúc & Phát triển (Architecture & Dev)](docs/architecture.md)** — Quyết định thiết kế, bản đồ module, và quy trình build / test / lint qua `Makefile`.

---

## Tổng quan nhanh

### Cơ chế hoạt động
1. **macOS Network Extension (`NETransparentProxyProvider` + `NEDNSProxyProvider`)**: Ở bản phát hành chính thức đã ký & công chứng, AnyProxy chặn toàn bộ lưu lượng TCP/UDP/DNS ở tầng hệ điều hành để ép mọi ứng dụng (kể cả app bỏ qua System Proxy) đi qua rule engine. Lần đầu bấm **Connect**, vào **System Settings → General → Login Items & Extensions**, cuộn xuống mục **Network Extensions** → bấm biểu tượng **`ⓘ`** bên phải → gạt **Bật** cho **AnyProxy Network Extension**.
2. **Windows WinDivert (`ProxyBridgeCore.dll` + `WinDivert`)**: Trên Windows (khi chạy cùng `ProxyBridgeCore.dll` / `WinDivert.dll` với quyền Administrator), AnyProxy bắt gói tin TCP/UDP ở tầng kernel cho mọi tiến trình.
3. **Local Inbound Proxy (`127.0.0.1:10800`) & OS System Proxy Fallback**: Luôn lắng nghe SOCKS5 (`CONNECT` + `UDP ASSOCIATE`) và HTTP/HTTPS (`CONNECT`), đồng thời tự động fallback về OS System Proxy (`networksetup` / Registry) khi chạy dev (`wails dev`) chưa nhúng extension/driver.

### Tính năng chính
- **Dual-mode Binary**: Mở không cờ để chạy Desktop GUI (Full & Compact Mini Mode); truyền cờ (`--profile`, `--list-profiles`, `--test-proxy`, `--verbose`, `--update`) để chạy Headless CLI.
- **Rule Engine ưu tiên từ trên xuống**: Khớp theo tên ứng dụng/tiến trình, domain/wildcard, IP/CIDR/dải IP, cổng, và giao thức với hành động `PROXY`, `DIRECT`, hoặc `BLOCK`; tự động bypass loopback/multicast/stub-DNS (`:53`).
- **5-Step Proxy Checker**: Kiểm tra TCP reachability, handshake & auth, HTTP probe, benchmark độ trễ 3 lần, và SOCKS5 UDP Associate.
- **Bảo mật dữ liệu**: File cấu hình `state.json` và `secrets.vault` (AES-256-GCM) đặt quyền owner-only (`0700`/`0600`), chỉ lưu khóa vault 32-byte trong OS Keychain / Credential Manager.
- **Tương thích `.pbprofile`**: Import/Export profile JSON chuẩn ProxyBridge (tùy chọn kèm hoặc ẩn mật khẩu) và tạo rule 1 chạm trực tiếp từ bảng **Live Flows** / **History Logs**.

---

## Bắt đầu nhanh

### Dùng CLI nhanh
```bash
# Xem toàn bộ tùy chọn CLI
anyproxy --help

# Liệt kê profile hiện có
anyproxy --list-profiles

# Chạy headless với profile đã lưu
anyproxy --profile "Default" --verbose 3

# Dùng trực tiếp local inbound listener với curl
curl --proxy socks5h://127.0.0.1:10800 https://api.ipify.org
```

### Build & Test từ mã nguồn
Yêu cầu **Go 1.26+**, **Node.js 18+**, và **Wails CLI v2** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

```bash
# Chạy toàn bộ test (Go race + TypeScript + Vitest)
make test

# Chạy GUI ở chế độ dev
make dev

# Build app bundle macOS (arm64) hoặc bộ cài Windows (amd64)
make build
make build-windows
```

Xem chi tiết các bước cài đặt, sử dụng và phát triển trong [docs/user-guide.md](docs/user-guide.md) và [docs/architecture.md](docs/architecture.md).

---

## Giấy phép

Phát hành theo giấy phép **AGPL-3.0-or-later**, ghi nhận đóng góp từ [InterceptSuite/ProxyBridge](https://github.com/InterceptSuite/ProxyBridge) (MIT) và AnyProfile (AGPL-3.0-or-later). Chi tiết xem tại [NOTICE](NOTICE).
