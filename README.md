# Hệ Thống Streaming Camera Android sang TV Box Armbian (Sử Dụng Docker & Go)

Hệ thống truyền tải luồng video trực tiếp từ điện thoại Android (qua cáp USB) lên TV Box chạy Armbian làm headless server, phân phối luồng cho các thiết bị xem từ xa thông qua giao thức WebRTC (độ trễ <0.5s), HLS hoặc RTSP/RTMP.

## Ưu điểm hệ thống
* **Siêu nhẹ & Tiết kiệm năng lượng**: Không giải mã/mã hóa lại (transcoding) trên TV Box. Luồng video được điện thoại tự mã hóa phần cứng và chuyển trực tiếp qua cáp USB. CPU TV Box hoạt động ở mức dưới 1%!
* **Không tốn băng thông Wi-Fi**: Dữ liệu truyền tải qua cáp USB thông qua ADB Reverse Tunnel, ổn định tuyệt đối và không gây trễ/lag do sóng Wi-Fi yếu.
* **Giao diện hiện đại (Web Dashboard)**: Dashboard thiết kế theo phong cách Glassmorphism (Dark Mode mặc định) hiển thị luồng video WebRTC trực tiếp, theo dõi thời gian thực pin/nhiệt độ điện thoại và CPU/RAM của TV Box.

---

## Yêu cầu chuẩn bị
1. **TV Box**: Chạy Armbian (hoặc bất kỳ OS Linux nào), RAM tối thiểu 2GB, đã được cài sẵn **Docker** và **Docker Compose**.
2. **Điện thoại Android**: Phiên bản Android bất kỳ, có bật chế độ **Gỡ lỗi USB (USB Debugging)**.
3. **Cáp USB**: Cáp kết nối chất lượng tốt giữa điện thoại và TV Box.

---

## Hướng dẫn cài đặt và vận hành nhanh

### Bước 1: Sao chép dự án lên TV Box
Tải hoặc copy toàn bộ thư mục `Stream_armbian` này vào TV Box của bạn.

### Bước 2: Khởi động hệ thống bằng Docker Compose
Mở terminal trên TV Box, di chuyển vào thư mục dự án và chạy lệnh:
```bash
docker compose up -d --build
```
*Lệnh này sẽ tự động build container Stream Manager từ mã nguồn Go và tải image MediaMTX chính thức về chạy ngầm.*

### Bước 3: Cấu hình trên Điện thoại Android
1. Truy cập **Cài đặt** trên điện thoại -> **Thông tin điện thoại** -> Ấn 7 lần vào **Số bản dựng (Build Number)** để kích hoạt Tùy chọn nhà phát triển.
2. Quay lại Cài đặt -> **Tùy chọn nhà phát triển** -> Bật **Gỡ lỗi USB (USB Debugging)**.
3. Cắm cáp USB nối điện thoại với TV Box. Trên màn hình điện thoại sẽ hiện bảng hỏi cấp quyền ADB từ TV Box, hãy tick chọn **"Luôn cho phép từ máy tính này"** rồi bấm **OK**.

### Bước 4: Xem Dashboard & Bắt đầu Live Stream
1. Sử dụng thiết bị bất kỳ trong mạng LAN/Tailscale mở trình duyệt web và truy cập:
   ```text
   http://<IP-CỦA-TV-BOX>:5000
   ```
2. Nếu kết nối USB thành công, Dashboard sẽ hiển thị thông tin Model điện thoại, % Pin và nhiệt độ của máy.
3. **Cài đặt ứng dụng camera trên điện thoại**:
   * Tải ứng dụng **Larix Broadcaster** từ CH Play (Miễn phí, cực kỳ ổn định).
   * Mở Larix -> Bấm biểu tượng bánh răng **Settings** -> **Connections** -> **New Connection**.
   * Nhập các thông tin:
     * **Name**: `USB Stream`
     * **URL**: `rtmp://127.0.0.1:1935/live/camera`
   * Bấm **Save**.
   * Quay lại màn hình chính của Larix, bấm nút **Ghi hình (Tròn đỏ)**.
4. **Xem kết quả**: Trình phát WebRTC trên Web Dashboard sẽ tự động nhận diện luồng và phát video ngay lập tức với độ trễ gần như bằng 0!

---

## Các giao thức và đường dẫn xem luồng (Dành cho các trình phát khác)

Nếu muốn xem luồng camera từ các phần mềm khác bên ngoài Dashboard:
* **WebRTC Player**: `http://<IP-CỦA-TV-BOX>:8889/live/camera` (Xem trực tiếp trên trình duyệt ngoài).
* **HLS (HTTP Live Streaming)**: `http://<IP-CỦA-TV-BOX>:8888/live/camera/index.m3u8` (Thích hợp cho Safari, Smart TV hoặc các ứng dụng đầu phát IPTV).
* **RTSP**: `rtsp://<IP-CỦA-TV-BOX>:8554/live/camera` (Xem qua VLC Player, PotPlayer, lưu trữ qua đầu ghi NVR).
* **RTMP**: `rtmp://<IP-CỦA-TV-BOX>:1935/live/camera`.

---

## Hướng dẫn sửa lỗi thường gặp (Troubleshooting)

### 1. Dashboard báo "Chưa kết nối USB" dù đã cắm cáp
* Đảm bảo bạn đã bật **Gỡ lỗi USB (USB Debugging)** trên điện thoại.
* Rút cáp ra cắm lại và kiểm tra xem màn hình điện thoại có hiện yêu cầu xác nhận quyền ADB hay không.
* Chạy thử lệnh kiểm tra trên TV Box host: `adb devices` (nếu host chưa cài adb có thể kiểm tra bằng lệnh `docker exec -it stream-manager adb devices`).

### 2. Stream bị giật lag hoặc mất kết nối liên tục
* Đảm bảo cáp USB truyền tải dữ liệu tốt (tránh cáp sạc dòng thấp hoặc bị lỏng đầu nối).
* Hạ độ phân giải trong Larix Broadcaster xuống 720p hoặc 1080p với bitrate từ 2000-4000 kbps để tối ưu hoá băng thông truyền tải của vi xử lý TV Box.
* Nếu điện thoại quá nóng (nhiệt độ pin hiển thị >45°C), hãy bấm nút **Tắt màn hình (Sleep)** trên Dashboard để giảm nhiệt độ của điện thoại (camera vẫn tiếp tục ghi và đẩy luồng bình thường ngay cả khi màn hình tắt!).

### 3. Không load được khung hình WebRTC trên trình duyệt
* Đảm bảo thiết bị của bạn truy cập đúng địa chỉ IP LAN của TV Box.
* Kiểm tra tường lửa trên TV Box có chặn cổng `8889` (WebRTC), `8888` (HLS) hoặc `8554` (RTSP) hay không.
