package repositories

import (
	"context"
	"fmt"
	"time"

	contractsdb "github.com/goravel/framework/contracts/database/db"
	"github.com/goravel/framework/support/carbon"

	"goravel/app/facades"
	"goravel/app/models"
)

const findDirectConversationSQL = `SELECT conversation_id
FROM direct_conversation_pairs
WHERE user_low_id = $1 AND user_high_id = $2
LIMIT 1`

const countDirectMessagesSQL = `SELECT COUNT(*) AS total
FROM messages
WHERE conversation_id = $1
  AND sender_id IN ($2, $3)
  AND message_type = 'TEXT'
  AND body IS NOT NULL
  AND BTRIM(body) <> ''
  AND status <> 'REMOVED'
  AND deleted_at IS NULL`

const listDirectMessagesSQL = `SELECT
	id,
	conversation_id,
	sender_id,
	COALESCE(body, '') AS body,
	created_at
FROM messages
WHERE conversation_id = $1
  AND sender_id IN ($2, $3)
  AND message_type = 'TEXT'
  AND body IS NOT NULL
  AND BTRIM(body) <> ''
  AND status <> 'REMOVED'
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $4 OFFSET $5`

const messageUsersSQL = `SELECT id, username, display_name, role, avatar_url, is_super_admin
FROM users
WHERE id IN ($1, $2)`

const insertDirectConversationSQL = `INSERT INTO conversations (
	type,
	created_by
) VALUES ('DIRECT', $1)
RETURNING id`

const upsertDirectConversationPairSQL = `INSERT INTO direct_conversation_pairs (
	conversation_id,
	user_low_id,
	user_high_id
) VALUES ($1, $2, $3)
ON CONFLICT (user_low_id, user_high_id)
DO UPDATE SET conversation_id = direct_conversation_pairs.conversation_id
RETURNING conversation_id`

const ensureDirectParticipantsSQL = `INSERT INTO conversation_participants (
	conversation_id,
	user_id,
	role
) VALUES
	($1, $2, 'MEMBER'),
	($1, $3, 'MEMBER')
ON CONFLICT (conversation_id, user_id)
DO UPDATE SET left_at = NULL`

const insertDirectMessageSQL = `INSERT INTO messages (
	conversation_id,
	sender_id,
	message_type,
	body,
	metadata,
	status
) VALUES ($1, $2, 'TEXT', $3, '{}'::jsonb, 'SENT')
RETURNING
	id,
	conversation_id,
	sender_id,
	COALESCE(body, '') AS body,
	created_at`

type directConversationRow struct {
	ConversationID string `db:"conversation_id"`
}

type insertedConversationRow struct {
	ID string `db:"id"`
}

type directMessageTotalRow struct {
	Total int64 `db:"total"`
}

type directMessageRow struct {
	ID             string    `db:"id"`
	ConversationID string    `db:"conversation_id"`
	SenderID       string    `db:"sender_id"`
	Body           string    `db:"body"`
	CreatedAt      time.Time `db:"created_at"`
}

type messageUserRow struct {
	ID           string  `db:"id"`
	Username     string  `db:"username"`
	DisplayName  string  `db:"display_name"`
	Role         string  `db:"role"`
	AvatarURL    *string `db:"avatar_url"`
	IsSuperAdmin bool    `db:"is_super_admin"`
}

// MessageRepository persists the REST direct-message API on top of the
// conversation-oriented Supabase schema.
type MessageRepository struct {
	database contractsdb.DB
}

func NewMessageRepository(database ...contractsdb.DB) *MessageRepository {
	if len(database) > 0 && database[0] != nil {
		return &MessageRepository{database: database[0]}
	}

	return &MessageRepository{database: facades.DB()}
}

func (r *MessageRepository) UserExists(ctx context.Context, userID string) (bool, error) {
	return r.database.WithContext(ctx).
		Table("users").
		Where("id", userID).
		Exists()
}

func (r *MessageRepository) ListConversation(
	ctx context.Context,
	currentUserID string,
	peerID string,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	database := r.database.WithContext(ctx)
	conversationID, found, err := findDirectConversation(
		database,
		currentUserID,
		peerID,
	)
	if err != nil {
		return nil, 0, err
	}
	if !found {
		return []models.Message{}, 0, nil
	}

	var totalRows []directMessageTotalRow
	if err = database.Select(
		&totalRows,
		countDirectMessagesSQL,
		conversationID,
		currentUserID,
		peerID,
	); err != nil {
		return nil, 0, err
	}
	if len(totalRows) != 1 {
		return nil, 0, fmt.Errorf("không đọc được tổng số tin nhắn")
	}

	rows := make([]directMessageRow, 0)
	if err = database.Select(
		&rows,
		listDirectMessagesSQL,
		conversationID,
		currentUserID,
		peerID,
		pageSize,
		(page-1)*pageSize,
	); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []models.Message{}, totalRows[0].Total, nil
	}

	users, err := loadMessageUsers(database, currentUserID, peerID)
	if err != nil {
		return nil, 0, err
	}
	messages, err := mapDirectMessages(rows, users, currentUserID, peerID)
	if err != nil {
		return nil, 0, err
	}

	return messages, totalRows[0].Total, nil
}

func (r *MessageRepository) Create(
	ctx context.Context,
	senderID string,
	receiverID string,
	body string,
) (models.Message, error) {
	database := r.database.WithContext(ctx)
	users, err := loadMessageUsers(database, senderID, receiverID)
	if err != nil {
		return models.Message{}, err
	}

	var created directMessageRow
	err = database.Transaction(func(tx contractsdb.Tx) error {
		conversationID, ensureErr := ensureDirectConversation(
			tx,
			senderID,
			receiverID,
		)
		if ensureErr != nil {
			return ensureErr
		}

		return tx.Select(
			&created,
			insertDirectMessageSQL,
			conversationID,
			senderID,
			body,
		)
	})
	if err != nil {
		return models.Message{}, err
	}

	messages, err := mapDirectMessages(
		[]directMessageRow{created},
		users,
		senderID,
		receiverID,
	)
	if err != nil {
		return models.Message{}, err
	}
	if len(messages) != 1 {
		return models.Message{}, fmt.Errorf("không đọc được tin nhắn vừa tạo")
	}

	return messages[0], nil
}

func findDirectConversation(
	source contractsdb.Tx,
	firstUserID string,
	secondUserID string,
) (string, bool, error) {
	lowUserID, highUserID := directConversationPair(firstUserID, secondUserID)
	rows := make([]directConversationRow, 0, 1)
	if err := source.Select(
		&rows,
		findDirectConversationSQL,
		lowUserID,
		highUserID,
	); err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}

	return rows[0].ConversationID, true, nil
}

func ensureDirectConversation(
	tx contractsdb.Tx,
	firstUserID string,
	secondUserID string,
) (string, error) {
	if conversationID, found, err := findDirectConversation(
		tx,
		firstUserID,
		secondUserID,
	); err != nil {
		return "", err
	} else if found {
		if err = ensureDirectConversationParticipants(
			tx,
			conversationID,
			firstUserID,
			secondUserID,
		); err != nil {
			return "", err
		}
		return conversationID, nil
	}

	inserted := insertedConversationRow{}
	if err := tx.Select(
		&inserted,
		insertDirectConversationSQL,
		firstUserID,
	); err != nil {
		return "", err
	}

	lowUserID, highUserID := directConversationPair(firstUserID, secondUserID)
	resolved := directConversationRow{}
	if err := tx.Select(
		&resolved,
		upsertDirectConversationPairSQL,
		inserted.ID,
		lowUserID,
		highUserID,
	); err != nil {
		return "", err
	}

	if resolved.ConversationID != inserted.ID {
		if _, err := tx.Delete(
			"DELETE FROM conversations WHERE id = $1",
			inserted.ID,
		); err != nil {
			return "", err
		}
	}
	if err := ensureDirectConversationParticipants(
		tx,
		resolved.ConversationID,
		firstUserID,
		secondUserID,
	); err != nil {
		return "", err
	}

	return resolved.ConversationID, nil
}

func ensureDirectConversationParticipants(
	tx contractsdb.Tx,
	conversationID string,
	firstUserID string,
	secondUserID string,
) error {
	return tx.Statement(
		ensureDirectParticipantsSQL,
		conversationID,
		firstUserID,
		secondUserID,
	)
}

func directConversationPair(firstUserID string, secondUserID string) (string, string) {
	if firstUserID < secondUserID {
		return firstUserID, secondUserID
	}

	return secondUserID, firstUserID
}

func loadMessageUsers(
	source contractsdb.Tx,
	firstUserID string,
	secondUserID string,
) (map[string]models.User, error) {
	rows := make([]messageUserRow, 0, 2)
	if err := source.Select(
		&rows,
		messageUsersSQL,
		firstUserID,
		secondUserID,
	); err != nil {
		return nil, err
	}

	users := make(map[string]models.User, len(rows))
	for _, row := range rows {
		users[row.ID] = models.User{
			BaseModel:    models.BaseModel{ID: row.ID},
			Username:     row.Username,
			DisplayName:  row.DisplayName,
			Role:         row.Role,
			AvatarURL:    row.AvatarURL,
			IsSuperAdmin: row.IsSuperAdmin,
		}
	}
	if _, ok := users[firstUserID]; !ok {
		return nil, ErrNotFound
	}
	if _, ok := users[secondUserID]; !ok {
		return nil, ErrNotFound
	}

	return users, nil
}

func mapDirectMessages(
	rows []directMessageRow,
	users map[string]models.User,
	currentUserID string,
	peerID string,
) ([]models.Message, error) {
	messages := make([]models.Message, 0, len(rows))
	for _, row := range rows {
		sender, ok := users[row.SenderID]
		if !ok {
			return nil, fmt.Errorf("người gửi không thuộc cuộc trò chuyện trực tiếp")
		}

		receiverID := currentUserID
		if row.SenderID == currentUserID {
			receiverID = peerID
		} else if row.SenderID != peerID {
			return nil, fmt.Errorf("người gửi không thuộc cuộc trò chuyện trực tiếp")
		}
		receiver, ok := users[receiverID]
		if !ok {
			return nil, fmt.Errorf("không đọc được người nhận tin nhắn")
		}

		messages = append(messages, models.Message{
			BaseModel: models.BaseModel{
				ID: row.ID,
				Timestamps: models.Timestamps{
					CreatedAt: carbon.NewDateTime(carbon.FromStdTime(row.CreatedAt)),
				},
			},
			ConversationID: row.ConversationID,
			SenderID:       row.SenderID,
			ReceiverID:     receiverID,
			Body:           row.Body,
			Sender:         sender,
			Receiver:       receiver,
		})
	}

	return messages, nil
}
