package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/goravel/framework/support/carbon"

	"goravel/app/models"
)

const (
	messageServiceTestUserOneID      = "00000000-0000-4000-8000-000000000001"
	messageServiceTestUserTwoID      = "00000000-0000-4000-8000-000000000002"
	messageServiceTestUnknownUserID  = "00000000-0000-4000-8000-000000000099"
	messageServiceTestListedMessage  = "50000000-0000-4000-8000-000000000001"
	messageServiceTestCreatedMessage = "50000000-0000-4000-8000-000000000004"
)

type fakeMessageRepository struct {
	users         map[string]bool
	userErr       error
	messages      []models.Message
	total         int64
	listErr       error
	createErr     error
	created       *models.Message
	listWasCalled bool
}

func (f *fakeMessageRepository) UserExists(_ context.Context, userID string) (bool, error) {
	if f.userErr != nil {
		return false, f.userErr
	}

	return f.users[userID], nil
}

func (f *fakeMessageRepository) ListConversation(
	_ context.Context,
	_ string,
	_ string,
	_ int,
	_ int,
) ([]models.Message, int64, error) {
	f.listWasCalled = true
	return f.messages, f.total, f.listErr
}

func (f *fakeMessageRepository) Create(
	_ context.Context,
	senderID string,
	receiverID string,
	body string,
) (models.Message, error) {
	if f.createErr != nil {
		return models.Message{}, f.createErr
	}

	message := models.Message{
		BaseModel:  models.BaseModel{ID: messageServiceTestCreatedMessage},
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

	repository := &fakeMessageRepository{users: map[string]bool{
		messageServiceTestUserTwoID: true,
	}}
	service := NewMessageService(repository)

	_, err := service.List(
		context.Background(),
		messageServiceTestUnknownUserID,
		messageServiceTestUserTwoID,
		1,
		50,
	)

	if !errors.Is(err, ErrDemoUserNotFound) {
		t.Fatalf("got error %v, want ErrDemoUserNotFound", err)
	}
	if repository.listWasCalled {
		t.Fatal("conversation query must not run for an unknown demo user")
	}
}

func TestMessageServiceIdentityFailureTakesPrecedenceOverSelfMessage(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[string]bool{}}
	service := NewMessageService(repository)

	_, err := service.Create(
		context.Background(),
		messageServiceTestUnknownUserID,
		messageServiceTestUnknownUserID,
		"Nội dung hợp lệ",
	)

	if !errors.Is(err, ErrDemoUserNotFound) {
		t.Fatalf("got error %v, want ErrDemoUserNotFound", err)
	}
	if repository.created != nil {
		t.Fatal("message must not be persisted for an unknown demo user")
	}
}

func TestMessageServiceListRejectsUnknownPeer(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[string]bool{
		messageServiceTestUserOneID: true,
	}}
	service := NewMessageService(repository)

	_, err := service.List(
		context.Background(),
		messageServiceTestUserOneID,
		messageServiceTestUnknownUserID,
		1,
		50,
	)

	if !errors.Is(err, ErrMessagePeerNotFound) {
		t.Fatalf("got error %v, want ErrMessagePeerNotFound", err)
	}
	if repository.listWasCalled {
		t.Fatal("conversation query must not run for an unknown peer")
	}
}

func TestMessageServiceListReturnsAnEmptyArrayForAnEmptyConversation(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[string]bool{
		messageServiceTestUserOneID: true,
		messageServiceTestUserTwoID: true,
	}}
	service := NewMessageService(repository)

	result, err := service.List(
		context.Background(),
		messageServiceTestUserOneID,
		messageServiceTestUserTwoID,
		1,
		50,
	)

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
		users: map[string]bool{
			messageServiceTestUserOneID: true,
			messageServiceTestUserTwoID: true,
		},
		messages: []models.Message{
			{
				BaseModel: models.BaseModel{
					ID: messageServiceTestListedMessage,
					Timestamps: models.Timestamps{
						CreatedAt: createdAt,
					},
				},
				SenderID:   messageServiceTestUserTwoID,
				ReceiverID: messageServiceTestUserOneID,
				Body:       "Em thử tăng độ tương phản ở vùng tiền cảnh nhé.",
				IsRead:     true,
				Sender: models.User{
					BaseModel:    models.BaseModel{ID: messageServiceTestUserTwoID},
					Username:     "co.mai",
					DisplayName:  "Cô Mai Anh",
					Role:         models.UserRoleTeacher,
					AvatarURL:    &avatarURL,
					IsSuperAdmin: true,
				},
				Receiver: models.User{
					BaseModel:   models.BaseModel{ID: messageServiceTestUserOneID},
					Username:    "linh.ve",
					DisplayName: "Nguyễn Gia Linh",
					Role:        models.UserRoleStudent,
				},
			},
		},
		total: 1,
	}
	service := NewMessageService(repository)

	result, err := service.List(
		context.Background(),
		messageServiceTestUserOneID,
		messageServiceTestUserTwoID,
		1,
		50,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalItems != 1 || len(result.Messages) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Messages[0].ID != messageServiceTestListedMessage ||
		result.Messages[0].Sender.ID != messageServiceTestUserTwoID ||
		result.Messages[0].Receiver.ID != messageServiceTestUserOneID {
		t.Fatalf("message and participant UUIDs were not mapped: %#v", result.Messages[0])
	}
	if result.Messages[0].CreatedAt != "2026-07-24T09:10:00+07:00" {
		t.Fatalf("got createdAt %q", result.Messages[0].CreatedAt)
	}
	if !result.Messages[0].Sender.IsSuperAdmin {
		t.Fatal("sender super-admin flag was not mapped")
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

	repository := &fakeMessageRepository{users: map[string]bool{
		messageServiceTestUserOneID: true,
	}}
	service := NewMessageService(repository)

	_, err := service.Create(
		context.Background(),
		messageServiceTestUserOneID,
		messageServiceTestUserOneID,
		"Tự nhắn",
	)

	if !errors.Is(err, ErrCannotMessageSelf) {
		t.Fatalf("got error %v, want ErrCannotMessageSelf", err)
	}
	if repository.created != nil {
		t.Fatal("self-message must not be persisted")
	}
}

func TestMessageServiceCreateRejectsUnknownRecipient(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[string]bool{
		messageServiceTestUserOneID: true,
	}}
	service := NewMessageService(repository)

	_, err := service.Create(
		context.Background(),
		messageServiceTestUserOneID,
		messageServiceTestUnknownUserID,
		"Xin chào",
	)

	if !errors.Is(err, ErrMessagePeerNotFound) {
		t.Fatalf("got error %v, want ErrMessagePeerNotFound", err)
	}
	if repository.created != nil {
		t.Fatal("message to an unknown recipient must not be persisted")
	}
}

func TestMessageServiceCreatePersistsValidatedMessage(t *testing.T) {
	t.Parallel()

	repository := &fakeMessageRepository{users: map[string]bool{
		messageServiceTestUserOneID: true,
		messageServiceTestUserTwoID: true,
	}}
	service := NewMessageService(repository)

	message, err := service.Create(
		context.Background(),
		messageServiceTestUserOneID,
		messageServiceTestUserTwoID,
		"Cô xem giúp em ạ.",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repository.created == nil {
		t.Fatal("expected message to be persisted")
	}
	if repository.created.SenderID != messageServiceTestUserOneID ||
		repository.created.ReceiverID != messageServiceTestUserTwoID {
		t.Fatalf("unexpected participants: %#v", repository.created)
	}
	if message.ID != messageServiceTestCreatedMessage || message.Body != "Cô xem giúp em ạ." {
		t.Fatalf("unexpected response message: %#v", message)
	}
}

func TestMessageServicePropagatesRepositoryFailure(t *testing.T) {
	t.Parallel()

	databaseErr := errors.New("database unavailable")
	repository := &fakeMessageRepository{
		users: map[string]bool{
			messageServiceTestUserOneID: true,
			messageServiceTestUserTwoID: true,
		},
		listErr: databaseErr,
	}
	service := NewMessageService(repository)

	_, err := service.List(
		context.Background(),
		messageServiceTestUserOneID,
		messageServiceTestUserTwoID,
		1,
		50,
	)

	if !errors.Is(err, databaseErr) {
		t.Fatalf("got error %v, want repository error", err)
	}
}
