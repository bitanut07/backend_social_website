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
- **PostgreSQL 17**; command-line client `psql` chỉ cần khi chạy local bằng CLI.
- OpenAI API key chỉ cần khi muốn bật adapter AI tùy chọn.

Backend này dùng PostgreSQL 17; cấu hình, migration, Docker Compose và
`sql.sql` đều thống nhất với PostgreSQL. Khóa chính và khóa ngoại của tài nguyên
dùng kiểu `UUID`; API biểu diễn chúng bằng chuỗi UUID, còn số trang, tổng số phần
tử và số reaction vẫn là số.

## Chạy nhanh với `sql.sql`

Từ thư mục `backend`:

```bash
go mod download
cp .env.example .env
go run . artisan key:generate
createdb -h 127.0.0.1 -U postgres artly_social
psql -h 127.0.0.1 -U postgres -d artly_social -f sql.sql
go run .
```

Nếu database đã tồn tại, bỏ qua lệnh `createdb`. Lệnh `key:generate` điền
`APP_KEY` 32 ký tự bắt buộc vào `.env`.

`sql.sql` là SQL PostgreSQL thuần, không chứa lệnh meta của `psql` và không tự
tạo hoặc đổi database. File tạo schema, bảng, index, trigger và dữ liệu mẫu
trong **database đang được kết nối**. Vì vậy phải tạo rồi chọn
`artly_social` trước khi chạy file. Script dùng `IF NOT EXISTS` và `ON CONFLICT`
nên có thể chạy lại trên schema UUID do chính file này tạo. Script **không tự
xóa bảng và không chuyển schema BIGINT cũ sang UUID**. Mặc định các bảng nằm
trong schema `public`; nếu dùng schema khác, hãy tạo schema đó trước và chạy
Goravel migration thay cho `sql.sql`.

Cấu hình mẫu dùng tài khoản PostgreSQL `postgres` và để trống mật khẩu để người
dùng điền theo server local. Nếu PostgreSQL trên máy dùng thông tin khác, thay
`postgres` trong lệnh và cập nhật `DB_USERNAME`, `DB_PASSWORD` trong `.env` cho
khớp. `psql` sẽ hỏi mật khẩu khi cơ chế xác thực của server yêu cầu. API mặc
định chạy tại `http://127.0.0.1:3000`.

Nếu tài khoản ứng dụng không có quyền `CREATEDB`, hãy nhờ quản trị viên tạo
database trước, rồi chạy file bằng tài khoản có quyền trên `artly_social`:

```bash
createdb -h 127.0.0.1 -U postgres -O artly_app artly_social
psql -h 127.0.0.1 -U artly_app -d artly_social -f sql.sql
```

Thay `artly_app` bằng role thực tế. Lệnh `createdb` ở ví dụ này phải do tài
khoản quản trị chạy; `artly_app` chỉ cần quyền tạo và sử dụng đối tượng trong
database đã được cấp cho mình.

### Chạy bằng Query Editor hoặc Supabase

Với Query Editor của một công cụ quản trị PostgreSQL thông thường:

1. Tạo database `artly_social` nếu chưa có.
2. Chọn/kết nối đúng database `artly_social`.
3. Mở `sql.sql`, dán toàn bộ nội dung vào editor rồi chạy.

Không dán lệnh shell `createdb` hoặc `psql` vào Query Editor.

Với **Supabase**, project đã có sẵn database và schema `public`, nên không cần
và không được chạy `createdb`/`psql` trong SQL Editor. Mở **SQL Editor** của
đúng project, tạo query mới, dán toàn bộ nội dung `sql.sql` rồi chọn **Run**.
Các bảng và dữ liệu mẫu UUID sẽ được tạo trong schema `public` của project đó.

#### Reset bản demo BIGINT cũ trên Supabase

Đổi ID số sang UUID là breaking change. Nếu project đã từng chạy bản `sql.sql`
dùng `BIGINT`, chạy lại file mới sẽ không sửa kiểu cột vì `CREATE TABLE IF NOT
EXISTS` giữ nguyên bảng cũ. Preflight trong file sẽ dừng sớm và báo bảng chưa
dùng UUID; kiểm tra thủ công bằng truy vấn sau:

```sql
SELECT data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'users'
  AND column_name = 'id';
```

Kết quả đúng cho schema hiện tại là `uuid`. Nếu kết quả là `bigint`, trước tiên
hãy sao lưu hoặc export mọi dữ liệu cần giữ và xác nhận đang mở đúng Supabase
project. Với database chỉ chứa dữ liệu demo có thể reset đúng các bảng Artly
bằng SQL sau; thao tác này xóa toàn bộ bài viết, reaction và tin nhắn hiện có:

```sql
BEGIN;
DROP TABLE IF EXISTS public.messages;
DROP TABLE IF EXISTS public.reactions;
DROP TABLE IF EXISTS public.post_topics;
DROP TABLE IF EXISTS public.posts;
DROP TABLE IF EXISTS public.topic_aliases;
DROP TABLE IF EXISTS public.topics;
DROP TABLE IF EXISTS public.users;
COMMIT;
```

Sau khi reset hoàn tất, dán và chạy lại toàn bộ `sql.sql`. Không drop schema
`public` vì schema này có thể chứa đối tượng khác của project. Nếu cần giữ dữ
liệu thật, không dùng đoạn reset trên; hãy xây dựng migration ánh xạ ID sau khi
đã backup và kiểm thử trên một project tạm.

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
createdb -h 127.0.0.1 -U postgres artly_social
go run . artisan migrate
```

Nếu database đã tồn tại, bỏ qua lệnh `createdb`. Migration chỉ tạo cấu trúc
database. Cần tự nạp dữ liệu hoặc chạy `sql.sql` sau đó nếu muốn dùng bộ dữ liệu
mẫu; file SQL không xóa dữ liệu đang có nhưng chỉ tương thích với schema UUID,
không tự nâng cấp bảng BIGINT cũ.

Migration `20260724000002_enforce_artly_uuid_schema` bảo đảm project đã ghi nhận
migration cũ vẫn kiểm tra thay đổi UUID. Nếu phát hiện cột khóa kiểu cũ,
`artisan migrate` sẽ dừng với hướng dẫn backup/reset. Sau khi reset các bảng
demo theo mục Supabase ở trên, chạy lại `artisan migrate`; migration tương thích
sẽ tạo lại schema UUID ngay cả khi ledger vẫn giữ migration `000001`.

### Chạy bằng Docker Compose (tùy chọn)

Sau khi đã tạo `.env` và `APP_KEY` như phần chạy nhanh:

```bash
docker compose up --build
```

Compose khởi động PostgreSQL 17. Biến `POSTGRES_DB` tạo database trước; sau đó
Docker chỉ dùng `sql.sql` để nạp schema, index và dữ liệu mẫu vào database đó
khi volume được tạo lần đầu. Compose chờ PostgreSQL sẵn sàng rồi mới chạy
Goravel tại cổng 3000. Tài khoản local mặc định là `postgres`, mật khẩu là
`artly_local`; có thể ghi đè bằng `DB_DATABASE`, `DB_USERNAME` và
`DB_PASSWORD` trong `.env`.

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
| `DB_CONNECTION` | `postgres` | Luôn dùng `postgres` cho dự án này. |
| `DB_HOST` | `127.0.0.1` | Máy chủ PostgreSQL. |
| `DB_PORT` | `5432` | Cổng PostgreSQL. |
| `DB_DATABASE` | `artly_social` | Database phải được tạo/chọn trước khi chạy `sql.sql`. |
| `DB_USERNAME` | `postgres` | Tài khoản PostgreSQL local/Compose. |
| `DB_PASSWORD` | trống | Mật khẩu PostgreSQL local; Compose fallback sang `artly_local`. |
| `DB_TIMEZONE` | `UTC` | Múi giờ của kết nối database. |
| `DB_SSLMODE` | `disable` | Tắt TLS cho local; production phải dùng chế độ phù hợp. |
| `DB_SCHEMA` | `public` | Schema PostgreSQL mặc định của ứng dụng. |
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
| `00000000-0000-4000-8000-000000000001` | `minh.an` | Trần Minh An | `STUDENT` |
| `00000000-0000-4000-8000-000000000002` | `co.lan` | Cô Nguyễn Hoài Lan | `TEACHER` |
| `00000000-0000-4000-8000-000000000003` | `phuong.thao` | Nguyễn Phương Thảo | `STUDENT` |

Seed có sáu chủ đề (Phong cảnh, Chân dung, Môi trường, Hòa bình, Di sản văn hóa
và Ước mơ), 12 alias, năm bài đăng (“Mầm xanh tương lai”, “Hòa bình trong em”,
“Di sản quê em”, “Chân dung người truyền cảm hứng” và “Thành phố trên mây”),
sáu reaction và bốn tin nhắn. Endpoint `GET /api/v1/users` vẫn là nguồn chính
xác để lấy ID dùng cho `X-User-ID` nếu dữ liệu trên máy đã được thay đổi.

Seed dùng UUID cố định để ví dụ và test có thể lặp lại: user bắt đầu bằng
`00000000`, topic bằng `10000000`, post bằng `20000000` và message bằng
`50000000`. ID tạo mới dùng `gen_random_uuid()`. Client phải giữ ID dưới dạng
chuỗi opaque, không ép sang số hoặc suy luận thứ tự từ UUID.

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

Ví dụ hỏi trợ lý với tài khoản mẫu Minh An:

```bash
curl -X POST http://127.0.0.1:3000/api/v1/assistant/questions \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 00000000-0000-4000-8000-000000000001" \
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
ánh xạ schema PostgreSQL. Tất cả giá trị động đi qua ORM/query builder hoặc
placeholder parameterized. Trợ lý chỉ tạo một ý định an toàn và chủ đề đã
resolve, không tạo câu truy vấn.

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
  database.go          Kết nối PostgreSQL duy nhất
database/
  migrations/          Schema có thể nâng cấp
docs/
  SPEC.md              Đặc tả backend
  openapi.yaml         Hợp đồng HTTP chính thức
routes/
  web.go               Route HTTP, gồm prefix /api/v1
sql.sql                SQL PostgreSQL thuần tạo schema, index và dữ liệu demo
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
