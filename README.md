# Artly Backend

REST API cho Artly — mạng xã hội bài thi vẽ dành cho học sinh và giáo viên.
Backend cung cấp tài khoản mẫu, chủ đề, bảng tin, reaction, tin nhắn trực tiếp
và trợ lý thống kê số bài viết theo chủ đề.

## Tính năng MVP

- Danh mục tài khoản mẫu và chủ đề vẽ.
- Bảng tin phân trang, lọc theo chủ đề và đăng bài bằng URL ảnh.
- Reaction idempotent: gọi thả hoặc gỡ nhiều lần vẫn cho cùng một trạng thái.
- Tin nhắn trực tiếp giữa hai tài khoản mẫu bằng REST polling.
- Trợ lý hiểu câu hỏi đếm bài theo chủ đề, dùng parser cục bộ tất định và có thể
  dùng OpenAI Responses API để đối chiếu tên chủ đề đã được allowlist.
- Một định dạng lỗi thống nhất và JSON camelCase trên toàn bộ API v1.

MVP chưa có xác thực production, upload file, WebSocket, stories, video, follow,
thông báo realtime hay kiến trúc microservice.

## Công nghệ và yêu cầu

- Go **1.25.12** hoặc **1.26.5 trở lên** để có đầy đủ bản vá bảo mật của
  toolchain; mã nguồn giữ mức ngôn ngữ `go 1.25.0` trong `go.mod`.
- Goravel 1.18.
- **MySQL 8** và MySQL command-line client.
- OpenAI API key chỉ cần khi muốn bật adapter AI tùy chọn.

Backend này chỉ hỗ trợ MySQL. Dự án không có cấu hình hay hướng dẫn PostgreSQL.

## Chạy nhanh với `sql.sql`

Từ thư mục `backend`:

```bash
go mod download
cp .env.example .env
go run . artisan key:generate
mysql -u root -p < sql.sql
go run .
```

Lệnh `key:generate` điền `APP_KEY` 32 ký tự bắt buộc vào `.env`. Lệnh `mysql`
sẽ hỏi mật khẩu, tạo database `artly_social` với UTF-8, tạo bảng/index và nạp
dữ liệu mẫu. File SQL dùng `IF NOT EXISTS`/upsert nên có thể chạy lại an toàn.
Nếu dùng tài khoản MySQL khác, thay `root` và cập nhật `DB_USERNAME`,
`DB_PASSWORD` trong `.env` cho khớp. API mặc định chạy tại
`http://127.0.0.1:3000`.

Kiểm tra dịch vụ:

```bash
curl http://127.0.0.1:3000/api/v1/health
```

Kết quả mong đợi:

```json
{"status":"OK"}
```

### Khởi tạo bằng migration (tùy chọn)

`sql.sql` là cách khuyến nghị cho bản demo vì tạo schema, index và dữ liệu mẫu
trong một lần. Nếu muốn quản lý schema qua Goravel migration:

```bash
mysql -u root -p -e \
  "CREATE DATABASE IF NOT EXISTS artly_social CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"
go run . artisan migrate
```

Migration chỉ tạo cấu trúc database. Cần tự nạp dữ liệu hoặc chạy `sql.sql` sau
đó nếu muốn dùng bộ dữ liệu mẫu; file SQL dùng `CREATE TABLE IF NOT EXISTS` và
upsert nên không xóa dữ liệu đang có.

### Chạy bằng Docker Compose (tùy chọn)

Sau khi đã tạo `.env` và `APP_KEY` như phần chạy nhanh:

```bash
docker compose up --build
```

Compose khởi động MySQL 8.4, tự nạp `sql.sql` khi volume database được tạo lần
đầu, chờ MySQL sẵn sàng rồi mới chạy Goravel tại cổng 3000. Mật khẩu MySQL local
mặc định của Compose là `artly_local`; có thể ghi đè bằng `DB_PASSWORD` trong
`.env`.

## Cấu hình

Sao chép `.env.example` thành `.env` và tối thiểu kiểm tra các biến sau:

| Biến | Mặc định mẫu | Ý nghĩa |
| --- | --- | --- |
| `APP_ENV` | `local` | Môi trường chạy ứng dụng. |
| `APP_KEY` | trống | Khóa 32 ký tự; tạo bằng `go run . artisan key:generate`. |
| `APP_DEBUG` | `true` | Chỉ bật khi phát triển; tắt ở production. |
| `APP_HOST` | `127.0.0.1` | Địa chỉ backend lắng nghe. |
| `APP_PORT` | `3000` | Cổng HTTP. |
| `CORS_ALLOWED_ORIGIN` | `http://localhost:5173` | Origin frontend được phép gọi API. |
| `DB_CONNECTION` | `mysql` | Luôn dùng `mysql` cho dự án này. |
| `DB_HOST` | `127.0.0.1` | Máy chủ MySQL. |
| `DB_PORT` | `3306` | Cổng MySQL. |
| `DB_DATABASE` | `artly_social` | Database được `sql.sql` tạo và chọn. |
| `DB_USERNAME` | `root` | Tài khoản kết nối MySQL. |
| `DB_PASSWORD` | trống | Mật khẩu MySQL trên máy local. |
| `DB_TIMEZONE` | `UTC` | Múi giờ của kết nối database. |
| `AI_PROVIDER` | `local` | Dùng `local` mặc định; đặt `openai` để thử adapter tùy chọn. |
| `OPENAI_API_KEY` | trống | Bỏ trống để chỉ dùng parser cục bộ. |
| `OPENAI_BASE_URL` | trống | Base URL tương thích OpenAI nếu cần ghi đè. |
| `AI_TEXT_MODEL` | `gpt-5.6-luna` | Model dùng cho adapter tùy chọn. |

Không commit `.env`, API key hay mật khẩu thật. `APP_KEY` phải được tạo riêng
cho từng môi trường. `JWT_SECRET` thuộc cấu hình nền của Goravel; cơ chế danh
tính MVP hiện không dùng JWT.

## OpenAI là tùy chọn

Với `AI_PROVIDER=local` hoặc khi không có `OPENAI_API_KEY`, trợ lý vẫn trả lời
các câu hỏi được hỗ trợ bằng parser cục bộ tất định. Khi đặt
`AI_PROVIDER=openai` cùng một key hợp lệ, backend vẫn tự nhận diện và allowlist
chủ đề trước. Adapter OpenAI chỉ nhận tên chủ đề chuẩn từ database; kết quả chỉ
được chấp nhận khi resolve lại đúng ID chủ đề ban đầu.

Nếu OpenAI timeout hoặc trả kết quả không hợp lệ, hệ thống tự fallback về parser
cục bộ. Mô hình không sinh hoặc thực thi SQL. Phép đếm cuối cùng luôn là truy
vấn parameterized do backend kiểm soát.

Ví dụ chỉ bật adapter trong môi trường local:

```dotenv
OPENAI_API_KEY=your-local-development-key
AI_PROVIDER=openai
AI_TEXT_MODEL=gpt-5.6-luna
```

## Tài khoản và dữ liệu mẫu

`sql.sql` chứa schema, index và dữ liệu seed để có thể dùng giao diện ngay sau
khi khởi tạo.

| ID dùng cho `X-User-ID` | Tài khoản | Tên hiển thị | Vai trò |
| --- | --- | --- | --- |
| `1` | `minh.an` | Trần Minh An | `STUDENT` |
| `2` | `co.lan` | Cô Nguyễn Hoài Lan | `TEACHER` |
| `3` | `phuong.thao` | Nguyễn Phương Thảo | `STUDENT` |

Seed có sáu chủ đề (Phong cảnh, Chân dung, Môi trường, Hòa bình, Di sản văn hóa
và Ước mơ), 12 alias, năm bài đăng (“Mầm xanh tương lai”, “Hòa bình trong em”,
“Di sản quê em”, “Chân dung người truyền cảm hứng” và “Thành phố trên mây”),
sáu reaction và bốn tin nhắn. Endpoint `GET /api/v1/users` vẫn là nguồn chính
xác để lấy ID dùng cho `X-User-ID` nếu dữ liệu trên máy đã được thay đổi.

```bash
curl "http://127.0.0.1:3000/api/v1/users?page=1&pageSize=20"
curl "http://127.0.0.1:3000/api/v1/topics?page=1&pageSize=20"
```

Dữ liệu mẫu liên kết với ba ảnh được tạo riêng trong frontend:

- `http://localhost:5173/demo-art/mam-xanh-tuong-lai.png`
- `http://localhost:5173/demo-art/di-san-que-em.png`
- `http://localhost:5173/demo-art/hoa-binh-trong-em.png`

Vì ảnh được Vite phục vụ, hãy chạy frontend ở cổng 5173 khi dùng nguyên dữ liệu
seed hoặc cập nhật các URL này theo origin frontend thực tế.

## API v1

Hợp đồng đầy đủ, schema request/response và ví dụ nằm tại
`docs/openapi.yaml`.

| Phương thức | Endpoint | `X-User-ID` | Công dụng |
| --- | --- | --- | --- |
| `GET` | `/api/v1/health` | Không | Kiểm tra trạng thái dịch vụ. |
| `GET` | `/api/v1/users` | Không | Lấy danh sách tài khoản mẫu. |
| `GET` | `/api/v1/topics` | Không | Lấy chủ đề và alias. |
| `GET` | `/api/v1/posts` | Có | Lấy bảng tin, phân trang và lọc `topicId`. |
| `POST` | `/api/v1/posts` | Có | Đăng bài bằng URL ảnh. |
| `PUT` | `/api/v1/posts/{id}/reaction` | Có | Thả reaction. |
| `DELETE` | `/api/v1/posts/{id}/reaction` | Có | Gỡ reaction. |
| `GET` | `/api/v1/messages` | Có | Đọc hội thoại với `peerId`. |
| `POST` | `/api/v1/messages` | Có | Gửi tin nhắn trực tiếp. |
| `POST` | `/api/v1/assistant/questions` | Có | Đếm bài viết theo chủ đề. |

Ví dụ hỏi trợ lý với tài khoản mẫu có ID `1`:

```bash
curl -X POST http://127.0.0.1:3000/api/v1/assistant/questions \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{"question":"Có bao nhiêu bài về chủ đề phong cảnh?"}'
```

Response danh sách có dạng:

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "pageSize": 10,
    "totalItems": 0,
    "totalPages": 0
  }
}
```

Mọi lỗi API có dạng:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Dữ liệu không hợp lệ",
    "details": {}
  }
}
```

## Các lệnh

| Lệnh | Công dụng |
| --- | --- |
| `go mod download` | Tải dependency đã khóa trong `go.mod`/`go.sum`. |
| `go run .` | Chạy HTTP server. |
| `go run . artisan route:list` | Liệt kê route đã đăng ký. |
| `go run . artisan migrate` | Chạy migration chưa áp dụng. |
| `go run . artisan migrate:rollback` | Hoàn tác đợt migration gần nhất. |
| `go test ./...` | Chạy toàn bộ test. |
| `go vet ./...` | Phân tích tĩnh mã Go. |

Lệnh kiểm tra đầy đủ trước khi bàn giao:

```bash
go test ./... && go vet ./...
```

## Kiến trúc

Controller xử lý HTTP và validation ở boundary; service chứa nghiệp vụ; model
ánh xạ schema MySQL. Tất cả giá trị động đi qua ORM/query builder hoặc placeholder
parameterized. Trợ lý chỉ tạo một ý định an toàn và chủ đề đã resolve, không tạo
câu truy vấn.

```text
app/
  http/
    controllers/       HTTP boundary và response API v1
    driver/             Giới hạn body/timeout trước khi Goravel đọc request
    middleware/         Lớp giới hạn body phòng thủ bổ sung
    responses/         Cấu trúc JSON thành công/lỗi dùng chung
    support/           Tiện ích đọc X-User-ID
  models/              Model Goravel ORM
  services/            Nghiệp vụ và parser/adapter trợ lý
bootstrap/
  migrations.go        Đăng ký migration với Goravel
config/
  ai.go                OpenAI adapter tùy chọn
  cors.go              CORS allowlist
  database.go          Kết nối MySQL duy nhất
database/
  migrations/          Schema có thể nâng cấp
docs/
  SPEC.md              Đặc tả backend
  openapi.yaml         Hợp đồng HTTP chính thức
routes/
  web.go               Route HTTP, gồm prefix /api/v1
sql.sql                Schema, index và dữ liệu demo cho MySQL 8
```

## Lưu ý bảo mật

`X-User-ID` chỉ là cơ chế chọn danh tính cho bản demo. Nó **không phải API key,
đăng nhập hay phân quyền production**; bất kỳ client nào cũng có thể tự đặt
header này. Trước khi triển khai thực tế phải thay bằng xác thực và ủy quyền đúng
chuẩn.

Ngoài ra:

- Không nối đầu vào người dùng vào SQL; luôn dùng tham số.
- Không thực thi SQL hoặc HTML do mô hình sinh ra.
- Không gửi secret hoặc dữ liệu nhận dạng cá nhân vào prompt.
- Response API đặt `Cache-Control: no-store, private`; các thao tác ghi được
  giới hạn 120 request/phút và trợ lý 20 request/phút theo địa chỉ IP.
- Lớp `net/http` chặn mọi body vượt 64 KiB trước khi Goravel parse JSON, đặt
  timeout request 3 giây và giới hạn thời gian đọc header/kết nối.
- Bản demo chạy trực tiếp nên bỏ các header IP proxy do client tự gửi. Khi đặt
  sau reverse proxy thật, cần cấu hình CIDR proxy tin cậy thay vì tin mọi
  `X-Forwarded-For`.
- Tắt debug, dùng CORS allowlist cụ thể và quản lý secret ngoài repository khi
  triển khai.
- Ảnh bài viết chỉ là URL HTTP/HTTPS; backend chưa nhận file upload.
