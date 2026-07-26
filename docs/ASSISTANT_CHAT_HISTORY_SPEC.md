# Đặc tả: Lịch sử trò chuyện Trợ lý Artly

## Mục tiêu

Mỗi tài khoản Artly có nhiều cuộc trò chuyện riêng với trợ lý. Người dùng có
thể bắt đầu chat mới, xem danh sách lịch sử theo lần hoạt động gần nhất, chọn
một cuộc trò chuyện cũ, đọc lại toàn bộ tin nhắn và tiếp tục đúng ngữ cảnh.
Lịch sử phải tồn tại sau khi reload trang hoặc đăng nhập lại bằng cùng tài
khoản mẫu.

Tiêu chí hành vi:

1. Nút **Chat mới** xóa vùng soạn thảo hiện tại và mở một hội thoại trống.
2. Hội thoại trống chưa được ghi database; câu hỏi đầu tiên tạo hội thoại và
   dùng nội dung câu hỏi làm tiêu đề rút gọn.
3. Mỗi lượt thành công lưu nguyên tử một tin nhắn `USER` và một tin nhắn
   `ASSISTANT`.
4. Chọn lịch sử chỉ đọc được hội thoại thuộc tài khoản hiện tại.
5. Khi tiếp tục hội thoại, backend tự lấy tối đa 4 cặp tin nhắn gần nhất làm
   context; frontend không phải gửi lại lịch sử đã lưu.
6. Danh sách lịch sử sắp xếp theo `updatedAt` giảm dần và cập nhật ngay sau một
   lượt chat thành công.
7. Có trạng thái loading, lỗi, rỗng và điều khiển được bằng bàn phím trên
   desktop/mobile.

Ngoài phạm vi hiện tại: đổi tên, xóa, chia sẻ hoặc tìm kiếm hội thoại.

## Công nghệ

- Backend: Go, Goravel, PostgreSQL 17.
- Frontend: React, TypeScript, Vite, Tailwind CSS.
- Model: Ollama qua SSH tunnel; backend vẫn là nguồn context và persistence
  duy nhất.

## Hợp đồng API

- `GET /api/v1/assistant/conversations?page=1&pageSize=30`
  trả danh sách hội thoại của tài khoản hiện tại.
- `GET /api/v1/assistant/conversations/{id}` trả metadata và toàn bộ tin nhắn
  của một hội thoại thuộc tài khoản hiện tại.
- `POST /api/v1/assistant/questions` nhận `question` và `conversationId` tùy
  chọn. `history` cũ vẫn được chấp nhận cho client stateless, nhưng khi có
  `conversationId`, context trong database là nguồn chính xác.
- Response của câu hỏi thêm `conversation` để client biết hội thoại vừa tạo
  hoặc vừa cập nhật.

Mọi JSON dùng camelCase và mọi lỗi giữ dạng
`{ "error": { "code", "message", "details" } }`.

## Cấu trúc dự án

```text
backend/app/models/                         Model conversation/message
backend/app/repositories/                   Persistence và ownership filter
backend/app/services/                       Điều phối AI + lịch sử
backend/app/http/controllers/               Endpoint assistant
backend/database/migrations/                Migration additive
backend/docs/openapi.yaml                   Hợp đồng HTTP
frontend/src/features/assistant/            Drawer, history rail, bubbles
frontend/src/lib/api.ts                     API client
frontend/src/types/api.ts                   Kiểu dữ liệu
```

## Quy ước code

```go
conversation, found, err := repository.GetConversation(ctx, userID, id)
if err != nil {
    return AssistantConversationDTO{}, err
}
if !found {
    return AssistantConversationDTO{}, ErrAssistantConversationNotFound
}
```

- Tên Go dùng PascalCase/camelCase theo codebase; JSON dùng camelCase.
- Không nối ID hoặc nội dung người dùng vào SQL; mọi truy vấn dùng placeholder.
- Component React tách phần lịch sử khỏi phần bong bóng chat.

## Chiến lược kiểm thử

- Unit test service: tạo hội thoại, tải context gần nhất, ownership, tiêu đề và
  lưu đúng cặp tin nhắn.
- Controller/API test: validation UUID, phân trang, 404 và response shape.
- Frontend component/App test: chat mới, tải danh sách, chọn lịch sử và gửi tiếp
  với `conversationId`.
- Browser E2E thủ công: gửi ở chat mới, reload, mở lại lịch sử, gửi nối tiếp.
- Quality gates:
  - `npm run lint`
  - `npm test -- --run`
  - `npm run build`
  - `go test ./...`
  - `go vet ./...`

## Ranh giới

- Luôn: kiểm tra ownership bằng `user_id`, dùng UUID, giới hạn độ dài và không
  đưa lịch sử của tài khoản khác vào model.
- Hỏi trước: thêm xóa/đổi tên/chia sẻ lịch sử hoặc đổi cơ chế xác thực.
- Không bao giờ: lưu secret vào message, thực thi code/SQL từ model, dùng
  conversation ID do client gửi mà không kiểm tra chủ sở hữu.

## Tiêu chí hoàn thành

- Database có hai bảng `assistant_chat_conversations` và
  `assistant_chat_messages` với FK, constraint và index phù hợp; tên riêng này
  không xung đột với các bảng assistant analytics của schema Supabase mở rộng.
- API list/show/ask hoạt động và không rò rỉ chéo tài khoản.
- UI có Chat mới, danh sách lịch sử và có thể chọn lại để tiếp tục.
- Reload trang vẫn thấy hội thoại vừa tạo.
- Toàn bộ quality gates và browser flow thực tế đều qua.

## Câu hỏi mở đã chốt

- Tiêu đề lấy từ câu hỏi đầu tiên, tối đa 80 ký tự.
- Tối đa 30 lịch sử trên một trang UI; API hỗ trợ phân trang.
- Chưa hỗ trợ rename/delete trong lát cắt này.
