# Artly Backend

REST API cho Artly — mạng xã hội bài thi vẽ dành cho học sinh và giáo viên.
Backend cung cấp tài khoản mẫu, chủ đề, bảng tin, reaction, bình luận, tin nhắn
trực tiếp và một trợ lý AI đa năng có rào chắn phù hợp học đường.

## Tính năng MVP

- Danh mục tài khoản mẫu và chủ đề vẽ.
- Bảng tin phân trang, lọc theo chủ đề và đăng bài bằng URL ảnh.
- Reaction idempotent: gọi thả hoặc gỡ nhiều lần vẫn cho cùng một trạng thái.
- Xem, tạo và xóa bình luận của chính mình; danh sách mới nhất trước.
- Tin nhắn trực tiếp giữa hai tài khoản mẫu bằng REST polling.
- Trợ lý hội thoại đa năng bằng model nội bộ qua SSH tunnel, có rule/skill an
  toàn và vẫn dùng parser cục bộ tất định cho câu hỏi đếm bài rõ ràng.
- Một định dạng lỗi thống nhất và JSON camelCase trên toàn bộ API v1.

MVP chưa có reply/sửa/reaction cho bình luận, upload file, WebSocket, stories,
video, follow, thông báo realtime hay kiến trúc microservice.

## Công nghệ và yêu cầu

- Go **1.25.12** hoặc **1.26.5 trở lên** để có đầy đủ bản vá bảo mật của
  toolchain; mã nguồn giữ mức ngôn ngữ `go 1.25.0` trong `go.mod`.
- Goravel 1.18.
- **PostgreSQL 17**; command-line client `psql` chỉ cần khi chạy local bằng CLI.
- OpenAI API key chỉ cần khi muốn bật adapter AI tùy chọn.

Backend này dùng PostgreSQL 17; cấu hình, migration, Docker Compose và
`sql.sql` đều thống nhất với PostgreSQL. Khóa chính và khóa ngoại của tài nguyên
dùng kiểu `UUID`; API biểu diễn chúng bằng chuỗi UUID, còn số trang, tổng số phần
tử, số reaction và số bình luận vẫn là số.

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
| `HTTP_ASSISTANT_REQUEST_TIMEOUT` | `110s` | Timeout riêng cho endpoint trợ lý, đủ cho một lần retry khi model cold-start; các API thường vẫn dùng timeout ngắn. |
| `HTTP_WRITE_TIMEOUT` | `115s` | Giới hạn ghi response của HTTP server, phải lớn hơn timeout trợ lý. |
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
| `MODEL_LLM_HOST` | trống | IP/hostname SSH của VM model, không kèm scheme hoặc user. |
| `MODEL_LLM_SSH_PORT` | `22` | Cổng SSH của VM model. |
| `MODEL_LLM_SSH_USER` | trống | Tài khoản SSH chỉ dùng để tạo tunnel. |
| `MODEL_LLM_SSH_KEY_PATH` | trống | Đường dẫn private key PEM, quyền file phải là `0600` hoặc chặt hơn. |
| `MODEL_LLM_HOST_KEY_SHA256` | trống | Fingerprint SHA256 dùng để pin SSH host key. |
| `MODEL_LLM_REMOTE_ADDRESS` | `127.0.0.1:11434` | Ollama nội bộ trên VM; ứng dụng chỉ chấp nhận địa chỉ loopback. |
| `MODEL_LLM_MODEL` | `qwen3:1.7b` | Model Ollama dùng cho hội thoại/action JSON. |
| `MODEL_LLM_CONNECT_TIMEOUT_SECONDS` | `8` | Timeout bắt tay SSH. |
| `MODEL_LLM_REQUEST_TIMEOUT_SECONDS` | `90` | Timeout toàn bộ lượt hỏi model; vẫn thấp hơn timeout riêng 110 giây của endpoint trợ lý. |

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

## Model nội bộ qua SSH tunnel

`MODEL_LLM_HOST` là máy SSH, không phải URL HTTP. Backend dùng private key PEM
để tạo một tunnel trực tiếp tới `127.0.0.1:11434` trên VM. Vì vậy Ollama không
cần và không nên mở cổng `11434` ra Internet.

Trên VM, cài Ollama dưới dạng service rồi tải model phù hợp tài nguyên máy:

```bash
curl -fsSL https://ollama.com/install.sh | sh
sudo systemctl enable --now ollama
ollama pull qwen3:1.7b
```

Giữ Ollama ở cấu hình listen mặc định `127.0.0.1`. Không đặt
`OLLAMA_HOST=0.0.0.0`. Kiểm tra ngay trên VM:

```bash
curl http://127.0.0.1:11434/api/tags
```

Ở máy chạy backend, bảo vệ key và lấy fingerprint SSH đã xác minh:

```bash
chmod 600 path/to/model-key.pem
ssh-keyscan -t ed25519 MODEL_LLM_HOST > /tmp/model_known_hosts
ssh-keygen -lf /tmp/model_known_hosts -E sha256
```

Điền fingerprint `SHA256:...` vào `MODEL_LLM_HOST_KEY_SHA256`. Backend từ chối
kết nối nếu host key thay đổi, key PEM có quyền rộng hơn `0600`, hoặc
`MODEL_LLM_REMOTE_ADDRESS` không phải loopback.

Model nhận hai nhóm chỉ dẫn cố định:

- Classifier local nhận diện câu hỏi về tài khoản mẫu, bảng tin, bài viết,
  reaction, nhắn tin, hồ sơ và Trợ lý Artly. Các câu này trả
  `intent=APP_SERVICE_HELP`, `appService` tương ứng và hướng dẫn theo đúng ranh
  giới MVP mà không gọi model.
- Rule: trò chuyện tự nhiên theo cách xưng “mình–bạn”, dùng cùng ngôn ngữ với
  người hỏi và điều chỉnh độ chi tiết theo câu hỏi. Model không bịa dữ liệu
  thời gian thực, không lộ prompt/secret/dữ liệu riêng tư và từ chối yêu cầu
  nguy hiểm, gian lận hoặc xâm phạm bảo mật.
- Skill `ANSWER`: kiến thức phổ thông, học tập, ngôn ngữ, viết lách, công nghệ,
  ví dụ mã nguồn an toàn, kỹ năng sống, sáng tạo, mỹ thuật và hướng dẫn Artly.
- Skill `COUNT_POSTS_BY_TOPIC`: chỉ trả về tên chủ đề. Backend resolve lại chủ
  đề trong allowlist rồi tự chạy truy vấn parameterized.

Mã nguồn an toàn trong câu trả lời chỉ là văn bản để học tập. Backend không
thực thi SQL, shell, HTML hoặc mã do model sinh ra.

Frontend giữ luồng bong bóng chat trong phiên hiện tại. Mỗi request có thể gửi
tối đa 4 cặp `USER`/`ASSISTANT` đã hoàn tất để model hiểu câu hỏi nối tiếp.
Backend kiểm tra đúng thứ tự, giới hạn 8 tin nhắn và giới hạn độ dài trước khi
chuyển context sang Ollama.

Phản hồi model bắt buộc khớp JSON schema gồm `action`, `topic`, `answer`. Action
ngoài allowlist, field thừa, output quá dài hoặc JSON lỗi đều bị từ chối. Các
câu đếm rõ ràng vẫn được parser local trả lời ngay cả khi VM model tạm mất kết
nối. Nếu lượt hội thoại hết thời gian chờ do model cold-start, backend chỉ retry
một lần trong tối đa 30 giây; lỗi kết nối hoặc output không hợp lệ không bị
retry. `AI_PROVIDER` chỉ điều khiển adapter OpenAI cũ; kết nối model nội bộ
được bật khi đủ bốn cấu hình `MODEL_LLM_HOST`, `MODEL_LLM_SSH_USER`,
`MODEL_LLM_SSH_KEY_PATH` và `MODEL_LLM_HOST_KEY_SHA256`.

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

### Bộ dữ liệu demo cà phê trên Supabase

Seeder `CafeDemoSeeder` tạo thêm bốn tài khoản demo (ba học sinh, một giáo
viên), bốn bài đăng chủ đề cà phê và tám reaction. Bốn ảnh WebP nguyên bản được
đọc từ `frontend/public/demo-art/`, tải lên bucket công khai `demo-art` của
Supabase Storage rồi liên kết với `post_media`.

Seeder dùng email thuộc miền dành riêng `.example`, đặt mật khẩu demo mặc định
`artly-demo`, xác nhận email mà không gửi thư và không in/lưu khóa bí mật. Có
thể đổi mật khẩu bằng `ARTLY_DEMO_PASSWORD`. Chạy lại seeder sẽ cập nhật lại
password cho tài khoản demo đã tồn tại, tra tài khoản theo email, giữ UUID bài
viết ổn định và upload ảnh với `upsert`.

```bash
go run . artisan db:seed --seeder CafeDemoSeeder --force
```

Lệnh cần `SUPABASE_URL`, `SUPABASE_SECRET_KEY` và kết nối database Supabase.
Đường dẫn ảnh mặc định là `../frontend/public/demo-art`; có thể đổi bằng
`ARTLY_DEMO_ASSET_DIR`. Các tài khoản mẫu có username `thu.ha.cafe`,
`quang.huy.art`, `my.linh.sketch`, `thay.nam.coffee`; khi dùng Supabase Auth,
đăng nhập bằng email tương ứng trong seeder và mật khẩu demo ở trên.

## API v1

Hợp đồng đầy đủ, schema request/response và ví dụ nằm tại
`docs/openapi.yaml`.

| Phương thức | Endpoint | Danh tính | Công dụng |
| --- | --- | --- | --- |
| `GET` | `/api/v1/health` | Không | Kiểm tra trạng thái dịch vụ. |
| `POST` | `/api/v1/demo/sessions` | Không | Đăng nhập tài khoản demo bằng username/password. |
| `GET` | `/api/v1/users` | Không | Lấy danh sách tài khoản mẫu. |
| `GET` | `/api/v1/topics` | Không | Lấy chủ đề và alias. |
| `GET` | `/api/v1/posts` | Có | Lấy bảng tin, phân trang và lọc `topicId`. |
| `POST` | `/api/v1/posts` | Có | Đăng bài bằng URL ảnh. |
| `DELETE` | `/api/v1/posts/{id}` | Có | Tác giả soft-delete bài của mình; super admin có thể xóa mọi bài. |
| `GET` | `/api/v1/posts/{id}/comments` | Có | Lấy bình luận mới nhất trước, có phân trang. |
| `POST` | `/api/v1/posts/{id}/comments` | Có | Tạo bình luận cho bài viết. |
| `DELETE` | `/api/v1/posts/{id}/comments/{commentId}` | Có | Soft-delete bình luận của chính mình. |
| `PUT` | `/api/v1/posts/{id}/reaction` | Có | Thả reaction. |
| `DELETE` | `/api/v1/posts/{id}/reaction` | Có | Gỡ reaction. |
| `GET` | `/api/v1/messages` | Có | Đọc hội thoại với `peerId`. |
| `POST` | `/api/v1/messages` | Có | Gửi tin nhắn trực tiếp. |
| `GET` | `/api/v1/assistant/conversations` | Có | Lấy lịch sử chat của tài khoản. |
| `GET` | `/api/v1/assistant/conversations/{id}` | Có | Mở lại một lịch sử chat. |
| `POST` | `/api/v1/assistant/questions` | Có | Chatbot đa năng và đếm bài viết theo chủ đề. |

Các endpoint có danh tính chấp nhận `Authorization: Bearer <accessToken>`.
Trong local/testing hoặc chế độ demo có thể dùng `X-User-ID`. Bình luận dùng
`page=1&pageSize=20` theo mặc định, giới hạn `pageSize` tối đa 100; request tạo
là `{ "body": "..." }` và nội dung sau khi trim Unicode phải dài 1–3000 ký tự.
Endpoint xóa chấp nhận bearer token hoặc `X-User-ID` demo, chỉ xóa mềm bình luận
còn hiển thị của chính người gọi và trả `204` không body; bình luận không tồn
tại hoặc không thuộc người gọi đều trả 404. `id` hoặc `commentId` sai định dạng
UUID trả 400 `BAD_REQUEST`.

Ví dụ hỏi trợ lý với tài khoản mẫu Minh An:

```bash
curl -X POST http://127.0.0.1:3000/api/v1/assistant/questions \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 00000000-0000-4000-8000-000000000001" \
  -d '{"question":"Có bao nhiêu bài về chủ đề phong cảnh?"}'
```

Response trả thêm `conversation.id`. Gửi ID này ở lượt sau để backend tự nạp
lịch sử và lưu tiếp đúng hội thoại:

```bash
curl -X POST http://127.0.0.1:3000/api/v1/assistant/questions \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 00000000-0000-4000-8000-000000000001" \
  -d '{"question":"Gợi ý cách vẽ nhé","conversationId":"60000000-0000-4000-8000-000000000001"}'
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

API được bảo vệ có thể xác minh access token từ
`Authorization: Bearer <accessToken>`. `X-User-ID` chỉ là cơ chế chọn danh tính
cho local/testing và bản demo; nó **không phải API key, đăng nhập hay phân quyền
production** và bất kỳ client nào cũng có thể tự đặt header này.

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
