# Đặc tả backend Artly

## Mục tiêu

Cung cấp REST API bằng Goravel cho mạng xã hội bài thi vẽ: người dùng mẫu, chủ
đề, bài đăng, reaction, tin nhắn trực tiếp và trợ lý thống kê. Backend mặc định
dùng PostgreSQL 17 và có `sql.sql` thuần PostgreSQL để tạo schema, index cùng dữ
liệu mẫu trong database đã được chọn.

## Công nghệ

- Go theo phiên bản được Goravel hỗ trợ.
- Goravel phiên bản ổn định hiện hành.
- PostgreSQL 17.
- OpenAI Responses API là adapter tùy chọn; parser cục bộ luôn sẵn sàng.

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
kết nối giữa các database.

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

- `users`: tài khoản mẫu với role `STUDENT`/`TEACHER`.
- `topics`, `topic_aliases`: chủ đề chuẩn hóa và cách gọi tương đương.
- `posts`, `post_topics`: bài đăng và quan hệ nhiều chủ đề.
- `reactions`: duy nhất theo `(post_id, user_id)`.
- `messages`: tin nhắn trực tiếp giữa sender và receiver.

## API v1

- `GET /api/v1/health`
- `GET /api/v1/users`
- `GET /api/v1/topics`
- `GET /api/v1/posts?page=1&pageSize=10&topicId=...`
- `POST /api/v1/posts`
- `PUT /api/v1/posts/{id}/reaction`
- `DELETE /api/v1/posts/{id}/reaction`
- `GET /api/v1/messages?peerId=...&page=1&pageSize=50`
- `POST /api/v1/messages`
- `POST /api/v1/assistant/questions`

`GET /posts`, hai endpoint `/messages`, các endpoint ghi dữ liệu và trợ lý yêu
cầu header `X-User-ID` trỏ tới user mẫu. Đây là cơ chế demo có chủ ý, không
phải auth production.

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

1. Chuẩn hóa câu hỏi, nhận diện intent đếm bài và resolve chủ đề/alias cục bộ.
2. Chỉ khi `AI_PROVIDER=openai` và có `OPENAI_API_KEY`, adapter OpenAI nhận tên
   chủ đề chuẩn đã được allowlist; không nhận nguyên văn câu hỏi.
3. Giá trị trả về được cắt độ dài, chuẩn hóa và chỉ được dùng khi resolve lại
   đúng ID chủ đề ban đầu.
4. Backend chạy truy vấn `COUNT(DISTINCT posts.id)` đã parameterize.
5. Mô hình không được sinh hoặc thực thi SQL.

## Kiểm thử

- Unit test cho normalize, parser câu hỏi và formatter câu trả lời.
- HTTP/service test cho validation, phân trang, reaction idempotent và quyền đọc
  tin nhắn.
- `go test ./...` và `go vet ./...` phải pass.

## Ranh giới

- Luôn: giới hạn độ dài input, CORS allowlist, timeout khi gọi AI, lỗi không lộ
  stack trace, secret chỉ lấy từ env.
- Hỏi trước: auth thật, upload file, thay schema, thêm dịch vụ AI khác.
- Không bao giờ: wildcard CORS production, nối input vào SQL, gửi secret/PII vào
  prompt, tin tưởng output LLM.

## Tiêu chí hoàn thành

- `sql.sql` chạy được bằng `psql` hoặc Query Editor với PostgreSQL 17; database
  đích phải được tạo/chọn trước, còn file tạo schema, index và dữ liệu mẫu.
- API đáp ứng đủ bài đăng, reaction, tin nhắn và thống kê chủ đề.
- Không cần API key vẫn trả lời được câu hỏi mẫu.
- Có API key thì dùng Responses API và fallback an toàn khi lỗi.
- Test/vet pass và README tiếng Việt mô tả đầy đủ cách chạy.
