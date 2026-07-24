package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/goravel/framework/support/carbon"

	"goravel/app/models"
)

type fakeMessageRepository struct {
	users         map[uint64]bool
	userErr       error
	messages      []models.Message
	total         int64
	listErr       error
	createErr     error
	created       *models.Message
	listWasCalled bool
}

func (f *fakeMessageRepository) UserExists(_ context.Context, userID uint64) (bool, error) {
	if f.userErr != nil {
		return false, f.userErr
	}

	return f.users[userID], nil
}

func (f *fakeMessageRepository) ListConversation(
	_ context.Context,
	_ uint64,
	_ uint64,
	_ int,
	_ int,
) ([]models.Message, int64, error) {
	f.listWasCalled = true
	return f.messages, f.total, f.listErr
}

func (f *fakeMessageRepository) Create(
	_ context.Context,
	senderID uint64,
	receiverID uint64,
	body string,
) (models.Message, error) {
	if f.createErr != nil {
		return models.Message{}, f.createErr
	}

	message := models.Message{
		BaseModel:  models.BaseModel{ID: 36},
		SenderID:   senderID,
		ReceiverID: receiverID,
		Body:       body,
		Sender:     models.User{BaseModel: models.BaseModel{ID: senderID}},
		Receiver:   models.User{BaseModel: models.BaseModel{ID: receiverID}},
	}
	f.created = &message

	return message, nil
}

func TestMessageServiceListRejectsUnknownDemoUser(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[uint64]bool{2: true}}
	service := NewMessageService(repository)

	_, err := service.List(context.Background(), 999, 2, 1, 50)

	if !errors.Is(err, ErrDemoUserNotFound) {
		t.Fatalf("got error %v, want ErrDemoUserNotFound", err)
	}
	if repository.listWasCalled {
		t.Fatal("conversation query must not run for an unknown demo user")
	}
}

func TestMessageServiceIdentityFailureTakesPrecedenceOverSelfMessage(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[uint64]bool{}}
	service := NewMessageService(repository)

	_, err := service.Create(context.Background(), 999, 999, "Nội dung hợp lệ")

	if !errors.Is(err, ErrDemoUserNotFound) {
		t.Fatalf("got error %v, want ErrDemoUserNotFound", err)
	}
	if repository.created != nil {
		t.Fatal("message must not be persisted for an unknown demo user")
	}
}

func TestMessageServiceListRejectsUnknownPeer(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[uint64]bool{1: true}}
	service := NewMessageService(repository)

	_, err := service.List(context.Background(), 1, 404, 1, 50)

	if !errors.Is(err, ErrMessagePeerNotFound) {
		t.Fatalf("got error %v, want ErrMessagePeerNotFound", err)
	}
	if repository.listWasCalled {
		t.Fatal("conversation query must not run for an unknown peer")
	}
}

func TestMessageServiceListReturnsAnEmptyArrayForAnEmptyConversation(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[uint64]bool{1: true, 2: true}}
	service := NewMessageService(repository)

	result, err := service.List(context.Background(), 1, 2, 1, 50)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Messages == nil {
		t.Fatal("empty conversation must be represented by an empty slice, not nil")
	}
	if len(result.Messages) != 0 || result.TotalItems != 0 {
		t.Fatalf("unexpected empty result: %#v", result)
	}
}

func TestMessageServiceListMapsOnlyPublicFieldsAndRFC3339Time(t *testing.T) {
	t.Parallel()

	avatarURL := "https://images.example.com/avatars/mai.jpg"
	createdAt := carbon.NewDateTime(carbon.Parse("2026-07-24 09:10:00", "Asia/Ho_Chi_Minh"))
	repository := &fakeMessageRepository{
		users: map[uint64]bool{1: true, 2: true},
		messages: []models.Message{
			{
				BaseModel: models.BaseModel{
					ID: 35,
					Timestamps: models.Timestamps{
						CreatedAt: createdAt,
					},
				},
				SenderID:   2,
				ReceiverID: 1,
				Body:       "Em thử tăng độ tương phản ở vùng tiền cảnh nhé.",
				IsRead:     true,
				Sender: models.User{
					BaseModel:   models.BaseModel{ID: 2},
					Username:    "co.mai",
					DisplayName: "Cô Mai Anh",
					Role:        models.UserRoleTeacher,
					AvatarURL:   &avatarURL,
				},
				Receiver: models.User{
					BaseModel:   models.BaseModel{ID: 1},
					Username:    "linh.ve",
					DisplayName: "Nguyễn Gia Linh",
					Role:        models.UserRoleStudent,
				},
			},
		},
		total: 1,
	}
	service := NewMessageService(repository)

	result, err := service.List(context.Background(), 1, 2, 1, 50)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalItems != 1 || len(result.Messages) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Messages[0].CreatedAt != "2026-07-24T09:10:00+07:00" {
		t.Fatalf("got createdAt %q", result.Messages[0].CreatedAt)
	}

	encoded, err := json.Marshal(result.Messages[0])
	if err != nil {
		t.Fatalf("marshal response DTO: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal response DTO: %v", err)
	}
	for _, internalField := range []string{"senderId", "receiverId", "isRead", "updatedAt"} {
		if _, exists := payload[internalField]; exists {
			t.Fatalf("response leaked internal field %q: %s", internalField, encoded)
		}
	}
}

func TestMessageServiceCreateRejectsSelfMessageWithoutWriting(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[uint64]bool{1: true}}
	service := NewMessageService(repository)

	_, err := service.Create(context.Background(), 1, 1, "Tự nhắn")

	if !errors.Is(err, ErrCannotMessageSelf) {
		t.Fatalf("got error %v, want ErrCannotMessageSelf", err)
	}
	if repository.created != nil {
		t.Fatal("self-message must not be persisted")
	}
}

func TestMessageServiceCreateRejectsUnknownRecipient(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[uint64]bool{1: true}}
	service := NewMessageService(repository)

	_, err := service.Create(context.Background(), 1, 404, "Xin chào")

	if !errors.Is(err, ErrMessagePeerNotFound) {
		t.Fatalf("got error %v, want ErrMessagePeerNotFound", err)
	}
	if repository.created != nil {
		t.Fatal("message to an unknown recipient must not be persisted")
	}
}

func TestMessageServiceCreatePersistsValidatedMessage(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[uint64]bool{1: true, 2: true}}
	service := NewMessageService(repository)

	message, err := service.Create(context.Background(), 1, 2, "Cô xem giúp em ạ.")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repository.created == nil {
		t.Fatal("expected message to be persisted")
	}
	if repository.created.SenderID != 1 || repository.created.ReceiverID != 2 {
		t.Fatalf("unexpected participants: %#v", repository.created)
	}
	if message.ID != 36 || message.Body != "Cô xem giúp em ạ." {
		t.Fatalf("unexpected response message: %#v", message)
	}
}

func TestMessageServicePropagatesRepositoryFailure(t *testing.T) {
	t.Parallel()

	databaseErr := errors.New("database unavailable")
	repository := &fakeMessageRepository{
		users:   map[uint64]bool{1: true, 2: true},
		listErr: databaseErr,
	}
	service := NewMessageService(repository)

	_, err := service.List(context.Background(), 1, 2, 1, 50)

	if !errors.Is(err, databaseErr) {
		t.Fatalf("got error %v, want repository error", err)
	}
}
