package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/goravel/framework/support/carbon"

	"goravel/app/models"
)

const (
	maximumAssistantConversationTitleLength = 80
	maximumStoredAssistantContextMessages   = 8
)

var (
	ErrAssistantConversationNotFound = errors.New("assistant conversation not found")
	ErrAssistantHistoryCorrupt       = errors.New("assistant history is invalid")
)

type AssistantHistoryRepository interface {
	UserExists(context.Context, string) (bool, error)
	ListConversations(
		context.Context,
		string,
		int,
		int,
	) ([]models.AssistantConversation, int64, error)
	GetConversation(
		context.Context,
		string,
		string,
	) (models.AssistantConversation, bool, error)
	ListMessages(context.Context, string) ([]models.AssistantMessage, error)
	ListRecentMessages(
		context.Context,
		string,
		int,
	) ([]models.AssistantMessage, error)
	SaveExchange(
		context.Context,
		string,
		string,
		string,
		string,
		string,
		[]byte,
	) (models.AssistantConversation, error)
}

type AssistantConversationAnswerer interface {
	AskConversation(
		context.Context,
		string,
		string,
		[]AssistantConversationMessage,
	) (AssistantResponse, error)
}

type AssistantConversationSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type AssistantStoredMessage struct {
	ID        string             `json:"id"`
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	Response  *AssistantResponse `json:"response,omitempty"`
	CreatedAt string             `json:"createdAt"`
}

type AssistantConversationDTO struct {
	AssistantConversationSummary
	Messages []AssistantStoredMessage `json:"messages"`
}

type AssistantConversationListResult struct {
	Conversations []AssistantConversationSummary
	TotalItems    int64
}

type AssistantHistoryService struct {
	repository AssistantHistoryRepository
	answerer   AssistantConversationAnswerer
}

func NewAssistantHistoryService(
	repository AssistantHistoryRepository,
	answerer AssistantConversationAnswerer,
) *AssistantHistoryService {
	return &AssistantHistoryService{
		repository: repository,
		answerer:   answerer,
	}
}

func (s *AssistantHistoryService) Ask(
	ctx context.Context,
	userID string,
	conversationID string,
	question string,
	legacyHistory []AssistantConversationMessage,
) (AssistantResponse, error) {
	if err := s.requireUser(ctx, userID); err != nil {
		return AssistantResponse{}, err
	}

	history := legacyHistory
	title := assistantConversationTitle(question)

	if conversationID != "" {
		conversation, found, err := s.repository.GetConversation(
			ctx,
			userID,
			conversationID,
		)
		if err != nil {
			return AssistantResponse{}, err
		}
		if !found {
			return AssistantResponse{}, ErrAssistantConversationNotFound
		}

		recent, err := s.repository.ListRecentMessages(
			ctx,
			conversation.ID,
			maximumStoredAssistantContextMessages,
		)
		if err != nil {
			return AssistantResponse{}, err
		}
		history, err = modelHistoryFromStoredMessages(recent)
		if err != nil {
			return AssistantResponse{}, err
		}
		title = conversation.Title
	}

	response, err := s.answerer.AskConversation(ctx, userID, question, history)
	if err != nil {
		return AssistantResponse{}, err
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return AssistantResponse{}, err
	}

	conversation, err := s.repository.SaveExchange(
		ctx,
		userID,
		conversationID,
		title,
		question,
		response.Answer,
		responseJSON,
	)
	if err != nil {
		return AssistantResponse{}, err
	}
	summary := assistantConversationSummaryFromModel(conversation)
	response.Conversation = &summary

	return response, nil
}

func (s *AssistantHistoryService) List(
	ctx context.Context,
	userID string,
	page int,
	pageSize int,
) (AssistantConversationListResult, error) {
	if err := s.requireUser(ctx, userID); err != nil {
		return AssistantConversationListResult{}, err
	}

	conversations, totalItems, err := s.repository.ListConversations(
		ctx,
		userID,
		page,
		pageSize,
	)
	if err != nil {
		return AssistantConversationListResult{}, err
	}

	items := make([]AssistantConversationSummary, len(conversations))
	for index := range conversations {
		items[index] = assistantConversationSummaryFromModel(conversations[index])
	}
	return AssistantConversationListResult{
		Conversations: items,
		TotalItems:    totalItems,
	}, nil
}

func (s *AssistantHistoryService) Get(
	ctx context.Context,
	userID string,
	conversationID string,
) (AssistantConversationDTO, error) {
	if err := s.requireUser(ctx, userID); err != nil {
		return AssistantConversationDTO{}, err
	}

	conversation, found, err := s.repository.GetConversation(
		ctx,
		userID,
		conversationID,
	)
	if err != nil {
		return AssistantConversationDTO{}, err
	}
	if !found {
		return AssistantConversationDTO{}, ErrAssistantConversationNotFound
	}

	messages, err := s.repository.ListMessages(ctx, conversation.ID)
	if err != nil {
		return AssistantConversationDTO{}, err
	}
	messageDTOs := make([]AssistantStoredMessage, len(messages))
	for index := range messages {
		messageDTOs[index], err = assistantStoredMessageFromModel(messages[index])
		if err != nil {
			return AssistantConversationDTO{}, err
		}
	}

	return AssistantConversationDTO{
		AssistantConversationSummary: assistantConversationSummaryFromModel(conversation),
		Messages:                     messageDTOs,
	}, nil
}

func (s *AssistantHistoryService) requireUser(
	ctx context.Context,
	userID string,
) error {
	exists, err := s.repository.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrDemoUserRequired
	}
	return nil
}

func assistantConversationTitle(question string) string {
	title := strings.Join(strings.Fields(strings.TrimSpace(question)), " ")
	runes := []rune(title)
	if len(runes) <= maximumAssistantConversationTitleLength {
		return title
	}

	return strings.TrimSpace(string(runes[:maximumAssistantConversationTitleLength-1])) + "…"
}

func modelHistoryFromStoredMessages(
	messages []models.AssistantMessage,
) ([]AssistantConversationMessage, error) {
	history := make([]AssistantConversationMessage, len(messages))
	for index, message := range messages {
		if (message.Role != "USER" && message.Role != "ASSISTANT") ||
			strings.TrimSpace(message.Content) == "" ||
			!utf8.ValidString(message.Content) {
			return nil, ErrAssistantHistoryCorrupt
		}
		history[index] = AssistantConversationMessage{
			Role:    message.Role,
			Content: message.Content,
		}
	}
	if len(history)%2 != 0 {
		return nil, ErrAssistantHistoryCorrupt
	}

	return history, nil
}

func assistantConversationSummaryFromModel(
	conversation models.AssistantConversation,
) AssistantConversationSummary {
	return AssistantConversationSummary{
		ID:        conversation.ID,
		Title:     conversation.Title,
		CreatedAt: assistantDateTimeString(conversation.CreatedAt),
		UpdatedAt: assistantDateTimeString(conversation.UpdatedAt),
	}
}

func assistantStoredMessageFromModel(
	message models.AssistantMessage,
) (AssistantStoredMessage, error) {
	stored := AssistantStoredMessage{
		ID:        message.ID,
		Role:      message.Role,
		Content:   message.Content,
		CreatedAt: assistantDateTimeString(message.CreatedAt),
	}
	if message.Role == "ASSISTANT" {
		var response AssistantResponse
		if len(message.ResponseJSON) == 0 ||
			json.Unmarshal(message.ResponseJSON, &response) != nil ||
			response.Answer != message.Content {
			return AssistantStoredMessage{}, ErrAssistantHistoryCorrupt
		}
		stored.Response = &response
	} else if message.Role != "USER" || len(message.ResponseJSON) != 0 {
		return AssistantStoredMessage{}, ErrAssistantHistoryCorrupt
	}

	return stored, nil
}

func assistantDateTimeString(value *carbon.DateTime) string {
	if value == nil || value.Carbon == nil || value.IsInvalid() {
		return ""
	}
	return value.ToRfc3339String()
}
