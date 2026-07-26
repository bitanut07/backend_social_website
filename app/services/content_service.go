package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"goravel/app/repositories"
)

var (
	ErrDemoUserNotFound       = errors.New("không tìm thấy tài khoản mẫu")
	ErrInvalidDemoCredentials = errors.New("tài khoản hoặc mật khẩu demo không hợp lệ")
	ErrNotFound               = repositories.ErrNotFound
	ErrForbidden              = repositories.ErrForbidden
)

const DemoPassword = "artly-demo"

type DemoLoginInput struct {
	Username string
	Password string
}

type MissingTopicsError struct {
	TopicIDs []string
}

func (e *MissingTopicsError) Error() string {
	return fmt.Sprintf("không tìm thấy chủ đề: %v", e.TopicIDs)
}

func (e *MissingTopicsError) Unwrap() error {
	return ErrNotFound
}

type ContentService struct {
	repository repositories.ContentRepository
}

func NewContentService(repository repositories.ContentRepository) *ContentService {
	return &ContentService{repository: repository}
}

func (s *ContentService) ListUsers(
	ctx context.Context,
	page, pageSize int,
) ([]repositories.User, int64, error) {
	return s.repository.ListUsers(ctx, page, pageSize)
}

func (s *ContentService) DemoLogin(
	ctx context.Context,
	input DemoLoginInput,
) (repositories.User, error) {
	username := normalizeDemoUsername(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password != DemoPassword {
		return repositories.User{}, ErrInvalidDemoCredentials
	}

	user, err := s.repository.UserByUsername(ctx, username)
	if errors.Is(err, repositories.ErrNotFound) {
		return repositories.User{}, ErrInvalidDemoCredentials
	}
	if err != nil {
		return repositories.User{}, err
	}

	return user, nil
}

func (s *ContentService) UpdateProfile(
	ctx context.Context,
	userID string,
	input repositories.UpdateProfileInput,
) (repositories.User, error) {
	if err := s.requireDemoUser(ctx, userID); err != nil {
		return repositories.User{}, err
	}

	return s.repository.UpdateProfile(ctx, userID, input)
}

func (s *ContentService) ListTopics(
	ctx context.Context,
	page, pageSize int,
) ([]repositories.Topic, int64, error) {
	return s.repository.ListTopics(ctx, page, pageSize)
}

func (s *ContentService) ListPosts(
	ctx context.Context,
	viewerID string,
	page, pageSize int,
	topicID, authorID *string,
) ([]repositories.Post, int64, error) {
	if err := s.requireDemoUser(ctx, viewerID); err != nil {
		return nil, 0, err
	}

	if topicID != nil {
		exists, err := s.repository.TopicExists(ctx, *topicID)
		if err != nil {
			return nil, 0, err
		}
		if !exists {
			return nil, 0, ErrNotFound
		}
	}

	if authorID != nil {
		exists, err := s.repository.UserExists(ctx, *authorID)
		if err != nil {
			return nil, 0, err
		}
		if !exists {
			return nil, 0, ErrNotFound
		}
	}

	return s.repository.ListPosts(ctx, viewerID, page, pageSize, topicID, authorID)
}

func (s *ContentService) CreatePost(
	ctx context.Context,
	userID string,
	input repositories.CreatePostInput,
) (repositories.Post, error) {
	if err := s.requireDemoUser(ctx, userID); err != nil {
		return repositories.Post{}, err
	}

	missingTopicIDs, err := s.repository.MissingTopicIDs(ctx, input.TopicIDs)
	if err != nil {
		return repositories.Post{}, err
	}
	if len(missingTopicIDs) > 0 {
		return repositories.Post{}, &MissingTopicsError{TopicIDs: missingTopicIDs}
	}

	return s.repository.CreatePost(ctx, userID, input)
}

func (s *ContentService) DeletePost(
	ctx context.Context,
	userID, postID string,
) error {
	if err := s.requireDemoUser(ctx, userID); err != nil {
		return err
	}

	return s.repository.DeletePost(ctx, userID, postID)
}

func (s *ContentService) PutReaction(
	ctx context.Context,
	userID, postID string,
) (repositories.ReactionState, error) {
	if err := s.requireDemoUser(ctx, userID); err != nil {
		return repositories.ReactionState{}, err
	}
	if err := s.requirePost(ctx, postID); err != nil {
		return repositories.ReactionState{}, err
	}

	return s.repository.PutReaction(ctx, userID, postID)
}

func (s *ContentService) DeleteReaction(
	ctx context.Context,
	userID, postID string,
) (repositories.ReactionState, error) {
	if err := s.requireDemoUser(ctx, userID); err != nil {
		return repositories.ReactionState{}, err
	}
	if err := s.requirePost(ctx, postID); err != nil {
		return repositories.ReactionState{}, err
	}

	return s.repository.DeleteReaction(ctx, userID, postID)
}

func (s *ContentService) requireDemoUser(ctx context.Context, userID string) error {
	exists, err := s.repository.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrDemoUserNotFound
	}

	return nil
}

func normalizeDemoUsername(username string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
}

func (s *ContentService) requirePost(ctx context.Context, postID string) error {
	exists, err := s.repository.PostExists(ctx, postID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	return nil
}
