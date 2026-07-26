package services

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"goravel/app/models"
)

const assistantHistoryTestConversationID = "60000000-0000-4000-8000-000000000001"

type assistantAnswererFake struct {
	response  AssistantResponse
	err       error
	calls     int
	questions []string
	histories [][]AssistantConversationMessage
}

func (f *assistantAnswererFake) AskConversation(
	_ context.Context,
	_ string,
	question string,
	history []AssistantConversationMessage,
) (AssistantResponse, error) {
	f.calls++
	f.questions = append(f.questions, question)
	f.histories = append(
		f.histories,
		append([]AssistantConversationMessage(nil), history...),
	)
	return f.response, f.err
}

type assistantHistoryRepositoryFake struct {
	userExists          bool
	userExistsErr       error
	conversation        models.AssistantConversation
	conversationFound   bool
	conversationErr     error
	recentMessages      []models.AssistantMessage
	recentErr           error
	savedConversation   models.AssistantConversation
	saveErr             error
	saveCalls           int
	savedUserID         string
	savedConversationID string
	savedTitle          string
	savedQuestion       string
	savedAnswer         string
	savedResponseJSON   []byte
}

func (f *assistantHistoryRepositoryFake) UserExists(
	context.Context,
	string,
) (bool, error) {
	return f.userExists, f.userExistsErr
}

func (f *assistantHistoryRepositoryFake) ListConversations(
	context.Context,
	string,
	int,
	int,
) ([]models.AssistantConversation, int64, error) {
	return nil, 0, nil
}

func (f *assistantHistoryRepositoryFake) GetConversation(
	context.Context,
	string,
	string,
) (models.AssistantConversation, bool, error) {
	return f.conversation, f.conversationFound, f.conversationErr
}

func (f *assistantHistoryRepositoryFake) ListMessages(
	context.Context,
	string,
) ([]models.AssistantMessage, error) {
	return nil, nil
}

func (f *assistantHistoryRepositoryFake) ListRecentMessages(
	context.Context,
	string,
	int,
) ([]models.AssistantMessage, error) {
	return f.recentMessages, f.recentErr
}

func (f *assistantHistoryRepositoryFake) SaveExchange(
	_ context.Context,
	userID string,
	conversationID string,
	title string,
	question string,
	answer string,
	responseJSON []byte,
) (models.AssistantConversation, error) {
	f.saveCalls++
	f.savedUserID = userID
	f.savedConversationID = conversationID
	f.savedTitle = title
	f.savedQuestion = question
	f.savedAnswer = answer
	f.savedResponseJSON = append([]byte(nil), responseJSON...)
	return f.savedConversation, f.saveErr
}

func TestAssistantHistoryServiceCreatesConversationOnFirstSuccessfulAnswer(t *testing.T) {
	t.Parallel()

	repository := &assistantHistoryRepositoryFake{
		userExists: true,
		savedConversation: models.AssistantConversation{
			BaseModel: models.BaseModel{ID: assistantHistoryTestConversationID},
			UserID:    assistantServiceTestUserID,
			Title:     "Vì sao bầu trời có màu xanh?",
		},
	}
	answerer := &assistantAnswererFake{
		response: AssistantResponse{
			Status:   AssistantStatusAnswered,
			Intent:   AssistantIntentChat,
			Answer:   "Ánh sáng xanh bị tán xạ trong khí quyển.",
			Provider: AssistantProviderModelLLM,
		},
	}
	service := NewAssistantHistoryService(repository, answerer)

	response, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		"",
		"Vì sao bầu trời có màu xanh?",
		nil,
	)

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if answerer.calls != 1 ||
		len(answerer.histories) != 1 ||
		len(answerer.histories[0]) != 0 {
		t.Fatalf("answerer histories = %#v", answerer.histories)
	}
	if repository.saveCalls != 1 ||
		repository.savedConversationID != "" ||
		repository.savedTitle != "Vì sao bầu trời có màu xanh?" ||
		repository.savedQuestion != "Vì sao bầu trời có màu xanh?" ||
		repository.savedAnswer != answerer.response.Answer {
		t.Fatalf("saved exchange = %#v", repository)
	}
	if response.Conversation == nil ||
		response.Conversation.ID != assistantHistoryTestConversationID {
		t.Fatalf("Ask() conversation = %#v", response.Conversation)
	}
}

func TestAssistantHistoryServiceLoadsStoredContextForExistingConversation(t *testing.T) {
	t.Parallel()

	repository := &assistantHistoryRepositoryFake{
		userExists:        true,
		conversationFound: true,
		conversation: models.AssistantConversation{
			BaseModel: models.BaseModel{ID: assistantHistoryTestConversationID},
			UserID:    assistantServiceTestUserID,
			Title:     "Màu nước cơ bản",
		},
		recentMessages: []models.AssistantMessage{
			{Role: "USER", Content: "Mình muốn thử màu nước."},
			{Role: "ASSISTANT", Content: "Bạn hãy chuẩn bị giấy và cọ."},
		},
		savedConversation: models.AssistantConversation{
			BaseModel: models.BaseModel{ID: assistantHistoryTestConversationID},
			UserID:    assistantServiceTestUserID,
			Title:     "Màu nước cơ bản",
		},
	}
	answerer := &assistantAnswererFake{
		response: AssistantResponse{
			Status:   AssistantStatusAnswered,
			Intent:   AssistantIntentChat,
			Answer:   "Bắt đầu với cọ tròn số 6 nhé.",
			Provider: AssistantProviderModelLLM,
		},
	}
	service := NewAssistantHistoryService(repository, answerer)

	_, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		assistantHistoryTestConversationID,
		"Nên chọn cọ nào?",
		[]AssistantConversationMessage{
			{Role: "USER", Content: "Context giả từ client."},
			{Role: "ASSISTANT", Content: "Không được dùng."},
		},
	)

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	wantHistory := []AssistantConversationMessage{
		{Role: "USER", Content: "Mình muốn thử màu nước."},
		{Role: "ASSISTANT", Content: "Bạn hãy chuẩn bị giấy và cọ."},
	}
	if !reflect.DeepEqual(answerer.histories[0], wantHistory) {
		t.Fatalf("model history = %#v, want %#v", answerer.histories[0], wantHistory)
	}
	if repository.savedConversationID != assistantHistoryTestConversationID ||
		repository.savedTitle != "Màu nước cơ bản" {
		t.Fatalf(
			"saved conversation = (%q, %q)",
			repository.savedConversationID,
			repository.savedTitle,
		)
	}
}

func TestAssistantHistoryServiceRejectsConversationOwnedByAnotherUser(t *testing.T) {
	t.Parallel()

	repository := &assistantHistoryRepositoryFake{
		userExists:        true,
		conversationFound: false,
	}
	answerer := &assistantAnswererFake{}
	service := NewAssistantHistoryService(repository, answerer)

	_, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		assistantHistoryTestConversationID,
		"Tiếp tục nhé",
		nil,
	)

	if !errors.Is(err, ErrAssistantConversationNotFound) {
		t.Fatalf("Ask() error = %v, want ErrAssistantConversationNotFound", err)
	}
	if answerer.calls != 0 || repository.saveCalls != 0 {
		t.Fatalf("unexpected calls: answerer=%d save=%d", answerer.calls, repository.saveCalls)
	}
}
