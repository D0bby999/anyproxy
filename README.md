# AnyProxyXa

[![Go Report Card](https://goreportcard.com/badge/github.com/D0bby999/anyproxyxa)](https://github.com/D0bby999/anyproxyxa)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**AnyProxyXa** là công cụ điều hướng lưu lượng mạng hệ thống (System-level Network Traffic Interceptor & Proxy Router) thuộc hệ sinh thái **AnyTools** (cùng với `anyprofile`, `anybrowser`, `any mobile`).

Dự án được xỉa và tái thiết kế hoàn toàn bằng **Go + Wails v2 + React/TypeScript** từ [InterceptSuite/ProxyBridge](https://github.com/InterceptSuite/ProxyBridge), bổ sung kiến trúc native driver hiện đại (macOS Network Extension & Windows WinDivert), mã hóa credential an toàn (OS Keychain / Credential Manager), cùng giao diện desktop chuẩn `framed-sheet` không phụ thuộc electron cồng kềnh.

---

## Tính năng chính

1. **Dual-Mode Binary (GUI & Headless CLI)**:
   - Chạy không cờ: Mở giao diện desktop native Wails v2 siêu nhẹ.
   - Chạy kèm cờ (`--profile`, `--list-profiles`, `--test-proxy`, `--verbose`): Chạy headless trực tiếp trên terminal/server hoặc tích hợp CI/CD scripts.
2. **Rule Engine mạnh mẽ**:
   - Khớp luồng theo Process name, Domain (exact, wildcard, regex), IP/CIDR, Port, Protocol (TCP/UDP).
   - Hỗ trợ Action: `DIRECT`, `BLOCK`, hoặc chuyển tiếp qua Proxy cụ thể.
   - Fallback bypass tự động cho DNS hệ thống (`127.0.0.53`, `::1`, multicast DNS) và local loopback để không bao giờ làm đơ mạng máy tính.
3. **5-Step Proxy Checker chuyên sâu**:
   - Tự động kiểm tra proxy trước khi sử dụng qua 5 bước nghiêm ngặt:
     1. TCP Handshake & Latency.
     2. Proxy Auth & Protocol Negotiation (SOCKS5/HTTP/HTTPS).
     3. Upstream HTTP GET (kiểm tra khả năng tải trang thực tế).
     4. DNS Resolution qua Proxy (phát hiện DNS leak).
     5. UDP Relay Capability (RFC 1928, kiểm tra hỗ trợ voice/game/WebRTC).
4. **Bảo mật chuẩn AnyProfile**:
   - Toàn bộ mật khẩu proxy được lưu trực tiếp vào OS Keychain (macOS) hoặc Credential Manager (Windows) qua `internal/secrets.Vault`.
   - File cấu hình và SQLite database được tạo với quyền hạn người dùng chặt chẽ (`0700` cho thư mục, `0600` cho file).
   - Tương thích 100% với định dạng file `.pbprofile` JSON chuẩn của ProxyBridge gốc.
5. **Drivers đa nền tảng**:
   - **macOS**: Swift System Extension (`macos/extension/`) sử dụng `NEAppProxyProvider` và `NEDNSProxyProvider`.
   - **Windows**: WinDivert packet filter service (`internal/driver/windows/`) kết hợp connection tracking (`conntrack`).
   - **Dev / Testing**: `MockDriver` cho phép chạy và kiểm thử toàn bộ UI & Logic mà không cần cấp quyền root kernel/system extension.

---

## Kiến trúc thư mục

```
anyproxyxa/
├── cmd/
├── internal/
│   ├── cli/            # Headless CLI runner
│   ├── dnsmap/         # Reverse DNS cache & IP-to-domain mapper
│   ├── driver/         # Driver abstraction & MockDriver
│   │   ├── darwin/     # macOS Extension IPC coordinator
│   │   └── windows/    # WinDivert packet filter & conntrack
│   ├── flows/          # Live flow tracker & circular activity log
│   ├── netproxy/       # SOCKS5/HTTP upstream relay & 5-step checker
│   ├── paths/          # OS-specific application data directories
│   ├── pbprofile/      # .pbprofile import/export serializer
│   ├── rules/          # Rule matcher engine (Domain/IP/Process/Port)
│   ├── secrets/        # OS Keychain / Credential Manager vault
│   ├── service/        # Engine state coordinator
│   └── store/          # modernc.org/sqlite database store
├── macos/extension/    # Swift Network Extension driver source
├── frontend/           # React 18 + TypeScript + Vite UI (Framed-sheet design)
├── main.go             # Application entrypoint (CLI or GUI router)
├── app.go              # Wails application lifecycle & Go bindings
└── Makefile            # Build targets & cross-compilation helpers
```

---

## Cài đặt & Build

### Yêu cầu hệ thống
- **Go**: 1.22 trở lên.
- **Node.js**: 18+ và `npm`.
- **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.

### Lệnh build

```bash
# 1. Cài đặt dependency frontend và build bundle
cd frontend && npm install && npm run build && cd ..

# 2. Chạy test toàn bộ backend Go
go test -race ./...

# 3. Build binary bất đối xứng (Dual GUI/CLI binary)
go build -o build/bin/anyproxyxa .
```

---

## Hướng dẫn sử dụng CLI

```bash
# Xem trợ giúp
./build/bin/anyproxyxa --help

# Liệt kê các profile hiện có
./build/bin/anyproxyxa --list-profiles

# Chạy headless với một profile đã lưu
./build/bin/anyproxyxa --profile "BurpSuite Intercept" --verbose 2

# Chạy headless trực tiếp từ file .pbprofile ngoài
./build/bin/anyproxyxa -p /path/to/custom.pbprofile

# Kiểm tra độ trễ và tính toàn vẹn của một proxy
./build/bin/anyproxyxa --test-proxy "proxy-uuid-hoac-ten"
```

---

## Tích hợp cùng AnyProfile / AnyTools

AnyProxyXa kết hợp hoàn hảo với hệ thống trình duyệt ẩn danh và anti-detect profile của **AnyProfile**:

- **Định tuyến độc lập từng Profile**: Định tuyến luồng mạng của từng tiến trình Chromium / Firefox riêng lẻ đi qua các proxy chain khác nhau mà không cần cấu hình extension proxy trên browser.
- **Pentest Bridge**: Chuyển tiếp toàn bộ traffic từ các app không hỗ trợ proxy (như game client, desktop app, terminal tool) đi xuyên qua Burp Suite hoặc Caido qua luật `Action: PROXY -> 127.0.0.1:8080`.
- **Chống rò rỉ DNS**: Toàn bộ yêu cầu DNS được DNS cache xử lý và chuyển tiếp qua upstream SOCKS5 an toàn.

---

## Giấy phép

Dự án này được phát hành theo giấy phép [MIT](LICENSE), ghi nhận bản quyền từ dự án gốc [InterceptSuite/ProxyBridge](https://github.com/InterceptSuite/ProxyBridge). Chi tiết xem tại file [NOTICE](NOTICE).
