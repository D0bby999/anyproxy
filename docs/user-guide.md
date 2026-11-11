# Hướng dẫn sử dụng AnyProxy

Tài liệu này hướng dẫn cách cài đặt, cấu hình và sử dụng **AnyProxy** ở cả hai chế độ: **Desktop GUI** và **Headless CLI**.

---

## 1. Cách AnyProxy hoạt động

AnyProxy vận hành theo mô hình **2 tầng**:

1. **Local Inbound Proxy (`127.0.0.1:10800`)** — Tự động lắng nghe ngay khi ứng dụng khởi động, hỗ trợ cả **SOCKS5** (`CONNECT`) và **HTTP / HTTPS (`CONNECT`)**. Mọi kết nối đi qua cổng này đều được dò tiến trình nguồn (`PID` và tên ứng dụng qua `netstat` + `ps`/`tasklist`) để áp dụng đúng **Routing Rules**.
2. **OS System Proxy (`Enable AnyProxy`)** — Khi bạn bấm **Enable AnyProxy** (hoặc chạy CLI), AnyProxy cấu hình System Proxy của hệ điều hành (qua `networksetup` trên macOS hoặc Registry `Internet Settings` trên Windows) trỏ HTTP, HTTPS và SOCKS5 về `127.0.0.1:10800`. Khi bấm **Disable AnyProxy** hoặc thoát app, cài đặt System Proxy được tự động tắt.

> **Mẹo:** Nếu bạn chỉ muốn định tuyến một trình duyệt hoặc công cụ cụ thể (như `curl`, AnyProfile) mà không đổi System Proxy toàn máy, bạn không cần bấm **Enable AnyProxy** — chỉ cần trỏ ứng dụng đó vào `127.0.0.1:10800`.

### Thứ tự quyết định luồng mạng

Mỗi kết nối đi qua `127.0.0.1:10800` được đánh giá theo thứ tự:

1. **Tự bỏ qua chính AnyProxy**: Lưu lượng do tiến trình `AnyProxy` tạo ra luôn đi `DIRECT` để tránh vòng lặp.
2. **Danh sách Rules (từ trên xuống dưới — khớp rule đầu tiên sẽ dừng)**:
   - Khớp theo **Protocol**, **Application/Process**, **Port**, và **Target Host/Domain**.
   - Nếu khớp rule `PROXY` nhưng đích là loopback/multicast/link-local/DHCP/stub-DNS (`127.0.0.53:53`), AnyProxy tự động chuyển về `DIRECT` để tránh đơ mạng máy tính (trừ loopback khi bật `Localhost via Proxy`).
3. **Fallback khi không khớp rule nào**:
   - Nếu đã chọn **Active Proxy (Default Proxy)** → chuyển tiếp qua proxy mặc định đó.
   - Nếu đặt là **Direct (No Proxy)** → kết nối thẳng ra Internet.
   - Nếu rule trỏ tới một proxy ID đã bị xóa/không tồn tại → **BLOCK** (không bao giờ âm thầm rò rỉ ra `DIRECT`).

---

## 2. Bắt đầu nhanh (Desktop GUI)

1. Mở **AnyProxy**.
2. Vào mục **Proxies** (`Máy chủ Proxy`) → bấm **Add Proxy** (hoặc dán danh sách ở khung **Add proxies**).
3. Bấm **Test** trên proxy vừa thêm để chạy kiểm tra 5 bước và xác nhận proxy hoạt động.
4. Bấm **Set Active** (hoặc bấm vào dòng proxy) để đặt làm **Active Proxy** mặc định (hoặc giữ `Direct` nếu bạn chỉ muốn proxy theo rule riêng).
5. Bấm nút **Enable AnyProxy** ở thanh bên trái để bật System Proxy toàn máy.
6. Mở **Connections** (`Kết nối mạng`) để quan sát lưu lượng thực tế.

---

## 3. Quản lý Proxy (`Proxies`)

### Thêm một proxy
- **Type**: `SOCKS5` hoặc `HTTP`.
- **Host / Port**: Địa chỉ và cổng của upstream proxy.
- **Username / Password** *(tùy chọn)*: Mật khẩu được mã hóa AES-256-GCM trong vault, không lưu dạng văn bản thuần.
- **Send Domain to Proxy**: Gửi tên miền thay vì IP đã phân giải cho upstream proxy (nên bật để tránh lộ DNS ở máy local khi upstream hỗ trợ).

### Thêm hàng loạt (Bulk Add)
Dán mỗi dòng một proxy vào ô nhập nhanh. Nếu không ghi giao thức, mặc định là `http`:

```text
http://user:password@1.2.3.4:8080
socks5://user:password@1.2.3.4:1080
1.2.3.4:8080:user:password
[2001:db8::1]:1080
```

### Đổi nhanh & Kiểm tra 5 bước (5-Step Proxy Checker)
- **Quick Switch**: Bấm **Set Active** ở bảng Proxies, hoặc dùng nút **Next Proxy** / **Ping All** ngay trên sidebar hoặc Compact Mode.
- **5 bước kiểm tra** (mặc định kiểm tra tới `httpbin.org:80` trên GUI hoặc `www.google.com:80` trên CLI):
  1. **TCP Reachability**: Mở kết nối TCP tới proxy server.
  2. **Tunnel & Authentication**: Bắt tay SOCKS5/HTTP CONNECT và xác thực.
  3. **Target Data Probe**: Gửi `HEAD / HTTP/1.1` qua tunnel và đọc phản hồi.
  4. **Latency Benchmark (3x)**: Đo độ trễ mở tunnel 3 lần liên tiếp (`min / avg / max` ms).
  5. **SOCKS5 UDP Associate**: Kiểm tra khả năng relay UDP (bỏ qua nếu là HTTP proxy; báo cảnh báo nếu SOCKS5 server chỉ hỗ trợ TCP).

---

## 4. Quy tắc định tuyến (`Routing Rules`)

Rules được duyệt **từ trên xuống dưới**; rule đầu tiên khớp tất cả điều kiện sẽ được áp dụng. Dùng nút **Move Up** / **Move Down** để sắp xếp độ ưu tiên, **Clone** để nhân bản, hoặc công tắc bật/tắt từng rule.

### Cú pháp điều kiện

Nhiều giá trị trong cùng một ô có thể phân tách bằng dấu `;` hoặc `,` (riêng mục Targets hỗ trợ thêm xuống dòng). Giá trị trống hoặc `*` nghĩa là khớp tất cả.

| Trường | Ví dụ hợp lệ | Ghi chú |
|---|---|---|
| **Applications** | `*`, `Google Chrome`, `curl`, `chrome.exe`, `/Applications/Telegram.app` | Không phân biệt hoa thường. Hỗ trợ wildcard `*` và `?`. Nếu nhập tên không có `/` hoặc `\`, chỉ khớp tên file/bundle (trên macOS tự rút gọn phần trước `.app`). Có nút **Browse App…** hoặc **Quick pick**. |
| **Targets** | `example.com`, `*.github.com`, `10.0.0.0/8`, `192.168.1.*`, `10.1-5.*.*`, `10.0.0.1-10.0.0.254`, `2001:db8::*` | Hỗ trợ domain chính xác, glob (`*`, `?`; lưu ý `*.example.com` khớp cả domain gốc `example.com`), IP chính xác, CIDR IPv4/IPv6, dải IP, dải octet IPv4, và tiền tố IPv6. |
| **Ports** | `*`, `443`, `80,443`, `8000-9000` | Cổng đơn lẻ hoặc khoảng cổng. |
| **Protocol** | `TCP & UDP`, `TCP only`, `UDP only` | Khi dùng qua local inbound (`127.0.0.1:10800`), lưu lượng hiện đi qua TCP. |
| **Action** | `Proxy`, `Direct`, `Block` | Với `Proxy`, có thể chọn một proxy cụ thể hoặc để **Default proxy** (đi theo Active Proxy đang chọn). |

### Import / Export Rules
- Trong trang **Routing Rules**, bấm **Export JSON** để chép mảng rules JSON vào clipboard, hoặc **Import JSON** để dán mảng rules JSON vào profile hiện tại.

---

## 5. Giám sát lưu lượng & Tạo rule nhanh (`Connections` & `Activity Logs`)

Trang **Connections** có 2 tab:
- **Live Flows**: Hiển thị các kết nối đang mở hoặc vừa đóng (Process, PID, Destination, Protocol, Action/Proxy, dung lượng `↑ Sent` / `↓ Received`, Status).
- **History Logs**: Lịch sử các kết nối đã ghi nhận.

**Tạo rule 1 chạm (Quick Rule)**: Trên mỗi dòng kết nối ở **Live Flows** hoặc **History Logs**, bạn có thể bấm trực tiếp:
- **Proxy App**: Tạo rule đưa toàn bộ lưu lượng của ứng dụng đó qua proxy.
- **Direct App**: Tạo rule cho ứng dụng đó đi thẳng (`DIRECT`).
- **Block Host**: Tạo rule chặn (`BLOCK`) tên miền/IP đích đó.

Trang **Activity Logs** hiển thị nhật ký khởi động listener, bật/tắt System Proxy, đổi profile và lỗi hệ thống.

---

## 6. Quản lý Profile, Cài đặt & Compact Mode

### Profile
Mỗi profile lưu danh sách **Proxies**, **Rules**, **Active Proxy**, **Log Filters**, và tùy chọn **Traffic & DNS** riêng biệt.
- Chuyển nhanh profile ở thanh bên trái hoặc trong **Settings → Profile Management** (`Create`, `Duplicate`, `Rename`, `Delete`).
- **Import / Export Profile JSON (`.pbprofile`)**: Tương thích định dạng `.pbprofile` của ProxyBridge (cả PascalCase và camelCase). Khi export, bạn có thể chọn **Include proxy passwords when exporting** nếu muốn mang mật khẩu sang máy khác (mặc định tắt để an toàn).

### Compact Mode (Chế độ thu gọn)
Bấm biểu tượng thu nhỏ trên tiêu đề sidebar để chuyển sang cửa sổ mini (`336×620`):
- Bật/tắt Engine, chuyển Profile, đổi Active Proxy 1 chạm, Ping từng proxy hoặc Ping All, chép `127.0.0.1:10800`, xem nhanh 3 flow gần nhất.
- Có nút **Pin on top** (ghim cửa sổ luôn nổi trên cùng) và lưu riêng tọa độ cửa sổ cho chế độ Full và Compact.

### Cài đặt khác (`Settings`)
- **Theme**: `System`, `Light`, `Dark`.
- **Language**: `English` hoặc `Tiếng Việt`.
- **Traffic & DNS**:
  - *Enable network traffic logging*: Bật/tắt ghi lịch sử kết nối.
  - *Forward DNS requests via selected proxy*: Gửi tên miền cho upstream xử lý (khi `SendDomainToProxy` bật).
  - *Log every connection without deduplication*: Ghi mọi kết nối thay vì gộp.
- **Updates & Version**: Kiểm tra phiên bản mới từ manifest (`latest.json`).

---

## 7. Sử dụng CLI (Headless Mode)

Binary `AnyProxy` là dual-mode: chạy **không cờ** sẽ mở giao diện GUI; chạy **kèm cờ** sẽ chạy headless trên terminal.

```text
Usage: anyproxy [options]

  -p, --profile <name|path|->   Nạp tên profile đã lưu, đường dẫn file .pbprofile, hoặc '-' (stdin)
      --list-profiles           Liệt kê các profile đã lưu (* đánh dấu active) và thoát
  -v, --verbose <0-3>           Mức log: 0=im lặng, 1=activity, 2=connections, 3=cả hai (mặc định: 1)
      --test-proxy <id|name>    Chạy 5-Step Proxy Checker cho proxy chỉ định và thoát
      --update                  Kiểm tra bản cập nhật mới
      --version                 In phiên bản và thoát
  -h, --help                    Hiển thị hướng dẫn sử dụng
```

### Ví dụ thường dùng

```bash
# Liệt kê profile hiện có
anyproxy --list-profiles

# Chạy headless với profile đã lưu, hiện log cả activity và connections
anyproxy --profile "Work" --verbose 3

# Nạp tạm từ file .pbprofile ngoài và chạy
anyproxy -p /path/to/profile.pbprofile

# Nạp profile JSON từ stdin
cat profile.pbprofile | anyproxy --profile -

# Kiểm tra nhanh một proxy theo tên hoặc ID
anyproxy --test-proxy "Office proxy"
```

Nhấn `Ctrl+C` (hoặc gửi `SIGINT`/`SIGTERM`) để dừng engine và tự động tắt System Proxy.

---

## 8. Kịch bản tích hợp thực tế

### Dùng chung với `curl` hoặc công cụ CLI
```bash
# Qua SOCKS5 (khuyến nghị socks5h để phân giải DNS qua proxy)
curl --proxy socks5h://127.0.0.1:10800 https://api.ipify.org

# Qua HTTP proxy
curl --proxy http://127.0.0.1:10800 https://api.ipify.org
```

### Dùng với Burp Suite / Caido (Pentest Bridge)
1. Thêm proxy `HTTP` trỏ tới `127.0.0.1:8080` (cổng listen của Burp/Caido).
2. Tạo rule `Action: Proxy` cho ứng dụng cần inspect (ví dụ `Telegram`, `Slack`, hoặc tên binary CLI) trỏ tới proxy vừa tạo.
3. Lưu ý: Nếu đích kiểm thử là `127.0.0.1`/`localhost`, bật `Localhost via Proxy` trên rule đó để không bị bypass về `DIRECT`.

### Dùng với AnyProfile / Trình duyệt tách biệt
Trỏ cấu hình proxy của từng trình duyệt hoặc profile về `socks5://127.0.0.1:10800` (hoặc `http://127.0.0.1:10800`), sau đó tạo rule theo tên tiến trình/domain trong AnyProxy để điều phối tập trung mà không cần cài extension trên từng trình duyệt.

---

## 9. Dữ liệu, Bảo mật & Lưu ý quan trọng

### Thư mục dữ liệu
- **macOS**: `~/Library/Application Support/AnyProxy/`
- **Windows**: `%APPDATA%\AnyProxy\`
- **Ghi đè thủ công**: Đặt biến môi trường `ANYPROXY_DATA_DIR` bằng đường dẫn tuyệt đối.
- Quyền thư mục được ép về owner-only (`0700` trên Unix).

Các file chính bên trong:
- `state.json`: Lưu profile, proxy configs (không gồm mật khẩu), rules, log filters và cài đặt cửa sổ (ghi atomic).
- `secrets.vault`: Lưu mật khẩu proxy đã mã hóa **AES-256-GCM**. Khóa 32-byte dùng để mở khóa file này được lưu riêng trong **macOS Keychain** hoặc **Windows Credential Manager**.

> **Nâng cấp từ phiên bản `AnyProxyXa` cũ:** Nếu chỉ tồn tại thư mục `AnyProxyXa`, bản mới sẽ tiếp tục đọc dữ liệu từ đó. Nếu muốn chuyển hẳn sang tên `AnyProxy`, hãy thoát hoàn toàn ứng dụng, sao lưu rồi đổi tên thư mục thủ công. Nếu cả hai thư mục cùng tồn tại, ứng dụng sẽ dừng để tránh chọn nhầm dữ liệu.

### Lưu ý về mạng & sự cố thường gặp
- **Cổng `127.0.0.1:10800` bị chiếm**: Đảm bảo không có tiến trình nào khác đang giữ cổng `10800`; kiểm tra tab **Activity Logs** để xem thông báo lỗi bind cổng.
- **Mất mạng sau khi app crash**: Nếu AnyProxy bị tắt đột ngột khi System Proxy đang bật, lần mở lại tiếp theo ứng dụng sẽ tự động phát hiện cờ `sysproxy_on` và tắt System Proxy giúp bạn (hoặc bạn có thể tắt thủ công trong cài đặt Proxy của macOS/Windows).
- **DNS Leak**: Khi dùng ở chế độ System Proxy, một số ứng dụng có thể tự phân giải DNS trước khi kết nối qua HTTP/SOCKS proxy hoặc bỏ qua System Proxy của hệ điều hành. Hãy luôn bật `SendDomainToProxy`, dùng `socks5h://` với CLI, và tự kiểm tra rò rỉ DNS theo nhu cầu bảo mật của bạn.
- **UDP**: Local inbound listener (`127.0.0.1:10800`) hiện chỉ nhận TCP (`SOCKS5 CONNECT` và `HTTP/HTTPS CONNECT`). Các ứng dụng dùng UDP thuần túy sẽ không đi qua System Proxy.

