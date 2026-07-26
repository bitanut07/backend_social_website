package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"goravel/app/facades"
	"goravel/app/models"
)

// AssistantHistoryRepository persists user-owned assistant conversations.
type AssistantHistoryRepository struct{}

func NewAssistantHistoryRepository() *AssistantHistoryRepository {
	return &AssistantHistoryRepository{}
}

func (r *AssistantHistoryRepository) UserExists(
	ctx context.Context,
	userID string,
) (bool, error) {
	return facades.Orm().
		WithContext(ctx).
		Query().
		Model(&models.User{}).
		Where("id = ?", userID).
		Exists()
}

func (r *AssistantHistoryRepository) ListConversations(
	ctx context.Context,
	userID string,
	page int,
	pageSize int,
) ([]models.AssistantConversation, int64, error) {
	conversations := make([]models.AssistantConversation, 0)
	var totalItems int64

	err := facades.Orm().
		WithContext(ctx).
		Query().
		Model(&models.AssistantConversation{}).
		Where("user_id = ?", userID).
		OrderByDesc("updated_at").
		OrderByDesc("id").
		Paginate(page, pageSize, &conversations, &totalItems)
	if err != nil {
		return nil, 0, err
	}
	return conversations, totalItems, nil
}

func (r *AssistantHistoryRepository) GetConversation(
	ctx context.Context,
	userID string,
	conversationID string,
) (models.AssistantConversation, bool, error) {
	var conversation models.AssistantConversation
	err := facades.Orm().
		WithContext(ctx).
		Query().
		Model(&models.AssistantConversation{}).
		Where("id = ? AND user_id = ?", conversationID, userID).
		FirstOrFail(&conversation)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return models.AssistantConversation{}, false, nil
	}
	if err != nil {
		return models.AssistantConversation{}, false, err
	}
	return conversation, true, nil
}

func (r *AssistantHistoryRepository) ListMessages(
	ctx context.Context,
	conversationID string,
) ([]models.AssistantMessage, error) {
	messages := make([]models.AssistantMessage, 0)
	err := facades.Orm().
		WithContext(ctx).
		Query().
		Model(&models.AssistantMessage{}).
		Where("conversation_id = ?", conversationID).
		OrderBy("position").
		Get(&messages)
	return messages, err
}

func (r *AssistantHistoryRepository) ListRecentMessages(
	ctx context.Context,
	conversationID string,
	limit int,
) ([]models.AssistantMessage, error) {
	messages := make([]models.AssistantMessage, 0, limit)
	err := facades.Orm().
		WithContext(ctx).
		Query().
		Model(&models.AssistantMessage{}).
		Where("conversation_id = ?", conversationID).
		OrderByDesc("position").
		Limit(limit).
		Get(&messages)
	if err != nil {
		return nil, err
	}

	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (r *AssistantHistoryRepository) SaveExchange(
	ctx context.Context,
	userID string,
	conversationID string,
	title string,
	question string,
	answer string,
	responseJSON []byte,
) (models.AssistantConversation, error) {
	var conversation models.AssistantConversation
	err := facades.Orm().WithContext(ctx).Transaction(func(query contractsorm.Query) error {
		if conversationID == "" {
			conversation = models.AssistantConversation{
				UserID: userID,
				Title:  title,
			}
			if err := query.Create(&conversation); err != nil {
				return err
			}
		} else {
			err := query.
				Model(&models.AssistantConversation{}).
				Where("id = ? AND user_id = ?", conversationID, userID).
				LockForUpdate().
				FirstOrFail(&conversation)
			if err != nil {
				return err
			}
		}

		messageCount, err := query.
			Model(&models.AssistantMessage{}).
			Where("conversation_id = ?", conversation.ID).
			Count()
		if err != nil {
			return err
		}
		lastPosition := int(messageCount)

		userMessage := models.AssistantMessage{
			ConversationID: conversation.ID,
			Position:       lastPosition + 1,
			Role:           "USER",
			Content:        question,
		}
		if err := query.Create(&userMessage); err != nil {
			return err
		}
		assistantMessage := models.AssistantMessage{
			ConversationID: conversation.ID,
			Position:       lastPosition + 2,
			Role:           "ASSISTANT",
			Content:        answer,
			ResponseJSON:   json.RawMessage(append([]byte(nil), responseJSON...)),
		}
		if err := query.Create(&assistantMessage); err != nil {
			return err
		}

		if _, err := query.
			Model(&models.AssistantConversation{}).
			Where("id = ? AND user_id = ?", conversation.ID, userID).
			Update("updated_at", time.Now().UTC()); err != nil {
			return err
		}

		return query.
			Model(&models.AssistantConversation{}).
			Where("id = ? AND user_id = ?", conversation.ID, userID).
			FirstOrFail(&conversation)
	})
	if err != nil {
		return models.AssistantConversation{}, err
	}
	return conversation, nil
}
