# Đặc tả backend Artly

## Mục tiêu

Cung cấp REST API bằng Goravel cho mạng xã hội bài thi vẽ: người dùng mẫu, chủ
đề, bài đăng, reaction, bình luận, tin nhắn trực tiếp và Trợ lý Artly. Backend
mặc định dùng PostgreSQL 17 và có `sql.sql` thuần PostgreSQL để tạo schema,
index cùng dữ liệu mẫu trong database đã được chọn. Mọi khóa tài nguyên công
khai dùng UUID; phân trang, số lượng và mã trạng thái HTTP vẫn dùng số.

## Công nghệ

- Go theo phiên bản được Goravel hỗ trợ.
- Goravel phiên bản ổn định hiện hành.
- PostgreSQL 17.
- OpenAI Responses API là adapter tùy chọn; parser cục bộ luôn sẵn sàng.
- Ollama trên VM nội bộ được truy cập qua SSH tunnel đã pin host key.

## Lệnh

- Cài dependency: `go mod tidy`
- Chạy server: `go run .`
- Liệt kê route: `go run . artisan route:list`
- Test: `go test ./...`
- Static analysis: `go vet ./...`

## Khởi tạo PostgreSQL

- Local CLI: tạo `artly_social` bằng `createdb`, sau đó chạy
  `psql -d artly_social -f sql.sql`.
- Query Editor thông thường: tạo và chọn `artly_social` trước, sau đó chạy toàn
  bộ nội dung `sql.sql`.
- Supabase SQL Editor: dùng database và schema `public` có sẵn của project, dán
  toàn bộ `sql.sql` rồi Run; không chạy lệnh shell `createdb` hoặc `psql`.
- Docker: `POSTGRES_DB` tạo database trước; entrypoint chỉ dùng `sql.sql` để tạo
  schema, index và dữ liệu seed trong database đó.

`sql.sql` không chứa lệnh meta của `psql`, không tạo database và không chuyển
kết nối giữa các database. File dùng `CREATE TABLE IF NOT EXISTS`, không tự drop
bảng và không chuyển schema `BIGINT` cũ sang `UUID`; preflight sẽ dừng sớm với
thông báo rõ ràng nếu phát hiện cột khóa PK/FK của schema Artly cũ chưa dùng
UUID.

### Breaking reset từ schema BIGINT cũ

Trước khi reset trên Supabase, phải backup/export dữ liệu cần giữ và xác nhận
đúng project. Có thể kiểm tra `public.users.id` trong
`information_schema.columns`; schema hiện tại phải có `data_type = 'uuid'`.
Nếu database demo cũ trả `bigint`, drop đúng bảy bảng Artly theo thứ tự
`messages`, `reactions`, `post_topics`, `posts`, `topic_aliases`, `topics`,
`users`, rồi chạy lại toàn bộ `sql.sql`. Không drop schema `public`. Quy trình
và câu lệnh reset đầy đủ nằm trong `README.md`; database có dữ liệu thật phải
dùng migration ánh xạ ID riêng thay vì reset.

## Cấu trúc

```text
app/http/controllers/  HTTP boundary và response
app/http/driver/       Body cap/timeout native trước khi Goravel parse request
app/http/middleware/   Lớp body cap phòng thủ bổ sung
app/models/            Goravel ORM models
app/services/          Nghiệp vụ và trợ lý
database/migrations/   Schema có thể nâng cấp
routes/                Route `/api/v1`
docs/openapi.yaml      Hợp đồng API
sql.sql                SQL PostgreSQL thuần tạo schema + index + seed
```

Migration tương thích `20260724000002_enforce_artly_uuid_schema` chạy cả với
ledger đã ghi nhận migration tạo bảng cũ. Nó từ chối schema khóa số với thông
báo backup/reset, và tạo lại schema UUID sau khi các bảng demo cũ đã được reset.

## Quy ước code

```go
func ErrorResponse(ctx http.Context, status int, code, message string, details any) http.Response {
	return ctx.Response().Json(status, map[string]any{
		"error": map[string]any{
			"code": code, "message": message, "details": details,
		},
	})
}
```

- Controller validate tại boundary; service nhận dữ liệu đã validate.
- ORM/query builder phải parameterize mọi giá trị.
- JSON response dùng camelCase; enum dùng UPPER_SNAKE_CASE.

## Data model

- Các PK/FK tài nguyên dùng kiểu PostgreSQL `UUID`, mặc định
  `gen_random_uuid()`. Seed dùng UUID v4 cố định để test và ví dụ lặp lại.
- `users`: tài khoản mẫu với role `STUDENT`/`TEACHER`.
- `topics`, `topic_aliases`: chủ đề chuẩn hóa và cách gọi tương đương.
- `posts`, `post_topics`: bài đăng và quan hệ nhiều chủ đề.
- `reactions`: duy nhất theo `(post_id, user_id)`.
- `comments`: bình luận phẳng gắn với một bài viết và tác giả.
- `messages`: tin nhắn trực tiếp giữa sender và receiver.

## API v1

- `GET /api/v1/health`
- `GET /api/v1/users`
- `GET /api/v1/topics`
- `GET /api/v1/posts?page=1&pageSize=10&topicId=10000000-0000-4000-8000-000000000001`
- `POST /api/v1/posts`
- `GET /api/v1/posts/20000000-0000-4000-8000-000000000001/comments?page=1&pageSize=20`
- `POST /api/v1/posts/20000000-0000-4000-8000-000000000001/comments`
- `DELETE /api/v1/posts/20000000-0000-4000-8000-000000000001/comments/30000000-0000-4000-8000-000000000001`
- `PUT /api/v1/posts/20000000-0000-4000-8000-000000000001/reaction`
- `DELETE /api/v1/posts/20000000-0000-4000-8000-000000000001/reaction`
- `GET /api/v1/messages?peerId=00000000-0000-4000-8000-000000000002&page=1&pageSize=50`
- `POST /api/v1/messages`
- `GET /api/v1/assistant/conversations?page=1&pageSize=30`
- `GET /api/v1/assistant/conversations/{id}`
- `POST /api/v1/assistant/questions`

Các endpoint được bảo vệ chấp nhận `Authorization: Bearer <accessToken>`. Khi
chạy local/testing hoặc chế độ demo, client có thể gửi `X-User-ID` trỏ tới UUID
user mẫu, ví dụ `00000000-0000-4000-8000-000000000001`; đây không phải auth
production.

`ResourceId` trong JSON, header, path và query là chuỗi có `format: uuid`.
Frontend phải xem ID là opaque string; không ép sang số. Các trường `page`,
`pageSize`, `totalItems`, `totalPages`, `reactionCount`, `commentCount`, `count`
và status HTTP tiếp tục là số.

Bình luận dùng `GET`/`POST /posts/{id}/comments`. Danh sách trả bình luận mới
nhất trước, mặc định `page=1&pageSize=20` và giới hạn `pageSize` tối đa 100.
Request tạo có dạng `{ "body": "..." }`; backend trim khoảng trắng Unicode rồi
yêu cầu từ 1 đến 3000 ký tự Unicode và không chứa U+0000. DTO bình luận gồm
`id`, `postId`, `body`, `author` (`User`) và `createdAt`; tạo thành công trả
`201` với `{ "data": ... }`. Hai endpoint dùng lỗi chuẩn
400/401/403/404/422/429/500.

`DELETE /posts/{id}/comments/{commentId}` chấp nhận bearer token hoặc
`X-User-ID` demo và chỉ soft-delete bình luận còn hiển thị của chính người gọi,
sau đó trả `204` không body. `id` hoặc `commentId` không phải UUID hợp lệ trả
400 `BAD_REQUEST`; bình luận không tồn tại, đã bị xóa, thuộc bài viết khác hoặc
không thuộc người gọi đều trả 404. Các lỗi còn lại là 401/429/500 và luôn dùng
error envelope chuẩn.

Response danh sách:

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

Response lỗi:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Dữ liệu không hợp lệ",
    "details": {}
  }
}
```

## Trợ lý

1. Câu đếm rõ ràng được parser local resolve chủ đề/alias và trả lời tất định.
2. Câu hỏi về tài khoản mẫu, bảng tin, bài viết, reaction, nhắn tin, hồ sơ hoặc
   chính Trợ lý Artly được classifier local nhận diện và trả hướng dẫn đúng
   ranh giới MVP với `intent=APP_SERVICE_HELP`.
3. Câu hội thoại hoặc câu cần suy luận được gửi qua SSH tunnel đến Ollama chỉ
   lắng nghe trên loopback của VM.
4. Frontend hiển thị hội thoại dạng bong bóng và gửi tối đa 4 cặp tin nhắn đã
   hoàn tất làm context. Backend kiểm tra role, thứ tự và độ dài của lịch sử.
5. System rule cho phép `ANSWER` xử lý kiến thức phổ thông, học tập, ngôn ngữ,
   viết lách, công nghệ, sáng tạo, trò chuyện và hướng dẫn Artly, đồng thời giữ
   rào chắn phù hợp học đường. Skill còn lại là `COUNT_POSTS_BY_TOPIC`.
6. Output phải khớp JSON schema. Backend từ chối action/field lạ, output lỗi
   hoặc quá dài.
7. Với skill đếm, backend resolve lại chủ đề trong allowlist rồi chạy
   `COUNT(DISTINCT posts.id)` đã parameterize.
8. Model có thể viết ví dụ mã nguồn an toàn dưới dạng văn bản, nhưng backend
   không thực thi SQL, shell, HTML hoặc mã do model sinh ra.

## Kiểm thử

- Unit test cho normalize, parser, SSH pinning, JSON schema và formatter.
- HTTP/service test cho validation, phân trang, thứ tự bình luận mới nhất trước,
  quyền xóa bình luận của chính mình, reaction idempotent và quyền đọc tin nhắn.
- `go test ./...` và `go vet ./...` phải pass.

## Ranh giới

- Luôn: giới hạn độ dài input, CORS allowlist, timeout khi gọi AI, lỗi không lộ
  stack trace, secret chỉ lấy từ env.
- Hỏi trước: auth thật, upload file, thay schema, thêm dịch vụ AI khác.
- Không bao giờ: wildcard CORS production, nối input vào SQL, gửi secret/PII vào
  prompt, tin tưởng output LLM.
- Bình luận MVP là danh sách phẳng và tác giả có thể xóa bình luận của chính
  mình; chưa có reply, sửa hoặc reaction cho bình luận.

## Tiêu chí hoàn thành

- `sql.sql` chạy được bằng `psql` hoặc Query Editor với PostgreSQL 17; database
  đích phải được tạo/chọn trước, còn file tạo schema UUID, index và dữ liệu mẫu.
- API đáp ứng đủ bài đăng, reaction, bình luận, tin nhắn và thống kê chủ đề.
- Không cần API key vẫn trả lời được câu hỏi mẫu.
- Có API key thì dùng Responses API và fallback an toàn khi lỗi.
- Test/vet pass và README tiếng Việt mô tả đầy đủ cách chạy.
