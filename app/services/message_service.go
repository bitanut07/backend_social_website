package services

import (
	"context"
	"errors"

	"goravel/app/models"
)

var (
	ErrMessagePeerNotFound = errors.New("message peer not found")
	ErrCannotMessageSelf   = errors.New("cannot send a message to the same user")
)

type MessageRepository interface {
	UserExists(ctx context.Context, userID string) (bool, error)
	ListConversation(
		ctx context.Context,
		currentUserID string,
		peerID string,
		page int,
		pageSize int,
	) ([]models.Message, int64, error)
	Create(ctx context.Context, senderID string, receiverID string, body string) (models.Message, error)
}

type MessageUserDTO struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	Role        string  `json:"role"`
	AvatarURL   *string `json:"avatarUrl"`
}

type MessageDTO struct {
	ID        string         `json:"id"`
	Sender    MessageUserDTO `json:"sender"`
	Receiver  MessageUserDTO `json:"receiver"`
	Body      string         `json:"body"`
	CreatedAt string         `json:"createdAt"`
}

type MessageListResult struct {
	Messages   []MessageDTO
	TotalItems int64
}

type MessageService struct {
	repository MessageRepository
}

func NewMessageService(repository MessageRepository) *MessageService {
	return &MessageService{repository: repository}
}

func (s *MessageService) List(
	ctx context.Context,
	currentUserID string,
	peerID string,
	page int,
	pageSize int,
) (MessageListResult, error) {
	if err := s.requireCurrentUser(ctx, currentUserID); err != nil {
		return MessageListResult{}, err
	}
	if currentUserID == peerID {
		return MessageListResult{}, ErrCannotMessageSelf
	}
	if err := s.requirePeer(ctx, peerID); err != nil {
		return MessageListResult{}, err
	}

	messages, totalItems, err := s.repository.ListConversation(
		ctx,
		currentUserID,
		peerID,
		page,
		pageSize,
	)
	if err != nil {
		return MessageListResult{}, err
	}

	responseMessages := make([]MessageDTO, len(messages))
	for index := range messages {
		responseMessages[index] = messageDTOFromModel(messages[index])
	}

	return MessageListResult{
		Messages:   responseMessages,
		TotalItems: totalItems,
	}, nil
}

func (s *MessageService) Create(
	ctx context.Context,
	currentUserID string,
	recipientID string,
	body string,
) (MessageDTO, error) {
	if err := s.requireCurrentUser(ctx, currentUserID); err != nil {
		return MessageDTO{}, err
	}
	if currentUserID == recipientID {
		return MessageDTO{}, ErrCannotMessageSelf
	}
	if err := s.requirePeer(ctx, recipientID); err != nil {
		return MessageDTO{}, err
	}

	message, err := s.repository.Create(ctx, currentUserID, recipientID, body)
	if err != nil {
		return MessageDTO{}, err
	}

	return messageDTOFromModel(message), nil
}

func (s *MessageService) requireCurrentUser(ctx context.Context, userID string) error {
	exists, err := s.repository.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrDemoUserNotFound
	}

	return nil
}

func (s *MessageService) requirePeer(ctx context.Context, userID string) error {
	exists, err := s.repository.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrMessagePeerNotFound
	}

	return nil
}

func messageDTOFromModel(message models.Message) MessageDTO {
	createdAt := ""
	if message.CreatedAt != nil && message.CreatedAt.Carbon != nil && !message.CreatedAt.IsInvalid() {
		createdAt = message.CreatedAt.ToRfc3339String()
	}

	return MessageDTO{
		ID:        message.ID,
		Sender:    messageUserDTOFromModel(message.Sender),
		Receiver:  messageUserDTOFromModel(message.Receiver),
		Body:      message.Body,
		CreatedAt: createdAt,
	}
}

func messageUserDTOFromModel(user models.User) MessageUserDTO {
	return MessageUserDTO{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		AvatarURL:   user.AvatarURL,
	}
}
