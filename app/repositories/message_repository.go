package repositories

import (
	"context"

	contractsorm "github.com/goravel/framework/contracts/database/orm"

	"goravel/app/facades"
	"goravel/app/models"
)

// MessageRepository persists direct messages and keeps all conversation
// filtering at the database boundary.
type MessageRepository struct{}

func NewMessageRepository() *MessageRepository {
	return &MessageRepository{}
}

func (r *MessageRepository) UserExists(ctx context.Context, userID string) (bool, error) {
	count, err := facades.Orm().
		WithContext(ctx).
		Query().
		Model(&models.User{}).
		Where("id", userID).
		Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *MessageRepository) ListConversation(
	ctx context.Context,
	currentUserID string,
	peerID string,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	messages := make([]models.Message, 0)
	var totalItems int64

	err := facades.Orm().
		WithContext(ctx).
		Query().
		Model(&models.Message{}).
		With("Sender").
		With("Receiver").
		Where(
			"(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			currentUserID,
			peerID,
			peerID,
			currentUserID,
		).
		OrderByDesc("created_at").
		OrderByDesc("id").
		Paginate(page, pageSize, &messages, &totalItems)
	if err != nil {
		return nil, 0, err
	}

	return messages, totalItems, nil
}

func (r *MessageRepository) Create(
	ctx context.Context,
	senderID string,
	receiverID string,
	body string,
) (models.Message, error) {
	message := models.Message{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Body:       body,
		IsRead:     false,
	}

	err := facades.Orm().WithContext(ctx).Transaction(func(query contractsorm.Query) error {
		if err := query.Create(&message); err != nil {
			return err
		}

		return query.
			With("Sender").
			With("Receiver").
			Where("id", message.ID).
			FirstOrFail(&message)
	})
	if err != nil {
		return models.Message{}, err
	}

	return message, nil
}
