# AnyProxy

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**AnyProxy** là công cụ điều hướng lưu lượng mạng hệ thống (System-level Network Traffic Interceptor & Proxy Router) thuộc hệ sinh thái **AnyTools** (cùng với `anyprofile`, `anybrowser`, `any mobile`).

Dự án được xỉa và tái thiết kế hoàn toàn bằng **Go + Wails v2 + React/TypeScript** từ [InterceptSuite/ProxyBridge](https://github.com/InterceptSuite/ProxyBridge), bổ sung kiến trúc native driver hiện đại (macOS Network Extension & Windows WinDivert), mã hóa credential an toàn (OS Keychain / Credential Manager), cùng giao diện desktop chuẩn `framed-sheet` không phụ thuộc electron cồng kềnh.

---

> **Trạng thái hiện tại:** Ứng dụng hỗ trợ proxy cục bộ SOCKS5/HTTP CONNECT tại `127.0.0.1:10800` cho các ứng dụng được cấu hình sử dụng địa chỉ này. Driver chặn lưu lượng toàn hệ thống trên macOS và Windows chưa được kết nối; không dùng ứng dụng này để đảm bảo chống rò rỉ DNS hoặc định tuyến bắt buộc toàn máy. Proxy cục bộ chỉ hỗ trợ TCP, không nhận diện tiến trình nguồn, và các rule không khớp sẽ đi trực tiếp. Listener vẫn chạy khi nút engine được tắt và chỉ dừng khi thoát ứng dụng.
>
> Ví dụ: `curl --proxy socks5h://127.0.0.1:10800 https://example.com`. Cần tạo rule PROXY phù hợp trong ứng dụng để chuyển tiếp qua proxy upstream; chỉ chọn proxy mặc định không tự động chuyển mọi kết nối qua nó.

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
     3. Upstream HTTP HEAD (kiểm tra phản hồi HTTP thực tế).
     4. Đo độ trễ thiết lập tunnel 3 lần (không phải kiểm tra DNS leak).
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
anyproxy/
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
go build -o build/bin/anyproxy .
```

---

Logo giao diện nằm tại `frontend/src/assets/anyproxy-mark.svg`; thiết kế icon ứng dụng riêng nằm tại `build/appicon.svg`. Sau khi sửa icon, tạo lại PNG cho Wails (cần `rsvg-convert`):

```bash
rsvg-convert -w 1024 -h 1024 build/appicon.svg -o build/appicon.png
```

## Dữ liệu từ phiên bản cũ

Bản mới dùng thư mục dữ liệu `AnyProxy`. Nếu chỉ có thư mục `AnyProxyXa` cũ, ứng dụng tiếp tục dùng dữ liệu đó mà không tự di chuyển trong khi ứng dụng cũ có thể đang chạy. Để đổi tên thư mục, thoát cả hai bản, sao lưu rồi đổi tên thư mục cũ thành `AnyProxy`. Tên database và định danh vault cũ được giữ nội bộ để đọc dữ liệu và mật khẩu hiện có. Nếu cả hai thư mục cùng tồn tại, ứng dụng dừng để tránh chọn nhầm. Biến `ANYPROXY_DATA_DIR` tiếp tục được hỗ trợ.

## Hướng dẫn sử dụng CLI

```bash
# Xem trợ giúp
./build/bin/anyproxy --help

# Liệt kê các profile hiện có
./build/bin/anyproxy --list-profiles

# Chạy headless với một profile đã lưu
./build/bin/anyproxy --profile "BurpSuite Intercept" --verbose 2

# Chạy headless trực tiếp từ file .pbprofile ngoài
./build/bin/anyproxy -p /path/to/custom.pbprofile

# Kiểm tra độ trễ và tính toàn vẹn của một proxy
./build/bin/anyproxy --test-proxy "proxy-uuid-hoac-ten"
```

---

## Tích hợp cùng AnyProfile / AnyTools

AnyProxy kết hợp hoàn hảo với hệ thống trình duyệt ẩn danh và anti-detect profile của **AnyProfile**:

- **Định tuyến độc lập từng Profile**: Định tuyến luồng mạng của từng tiến trình Chromium / Firefox riêng lẻ đi qua các proxy chain khác nhau mà không cần cấu hình extension proxy trên browser.
- **Pentest Bridge**: Chuyển tiếp toàn bộ traffic từ các app không hỗ trợ proxy (như game client, desktop app, terminal tool) đi xuyên qua Burp Suite hoặc Caido qua luật `Action: PROXY -> 127.0.0.1:8080`.
- **DNS**: Chưa có bảo đảm chống rò rỉ DNS toàn hệ thống; cấu hình ứng dụng khách và proxy phù hợp, rồi tự kiểm tra trước khi sử dụng.

---

## Giấy phép

Dự án này được phát hành theo giấy phép [MIT](LICENSE), ghi nhận bản quyền từ dự án gốc [InterceptSuite/ProxyBridge](https://github.com/InterceptSuite/ProxyBridge). Chi tiết xem tại file [NOTICE](NOTICE).

