package services

import (
	"context"
	"errors"
	"fmt"

	"goravel/app/repositories"
)

var (
	ErrDemoUserNotFound = errors.New("không tìm thấy tài khoản mẫu")
	ErrNotFound         = repositories.ErrNotFound
)

type MissingTopicsError struct {
	TopicIDs []int64
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

func (s *ContentService) ListTopics(
	ctx context.Context,
	page, pageSize int,
) ([]repositories.Topic, int64, error) {
	return s.repository.ListTopics(ctx, page, pageSize)
}

func (s *ContentService) ListPosts(
	ctx context.Context,
	viewerID int64,
	page, pageSize int,
	topicID *int64,
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

	return s.repository.ListPosts(ctx, viewerID, page, pageSize, topicID)
}

func (s *ContentService) CreatePost(
	ctx context.Context,
	userID int64,
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

func (s *ContentService) PutReaction(
	ctx context.Context,
	userID, postID int64,
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
	userID, postID int64,
) (repositories.ReactionState, error) {
	if err := s.requireDemoUser(ctx, userID); err != nil {
		return repositories.ReactionState{}, err
	}
	if err := s.requirePost(ctx, postID); err != nil {
		return repositories.ReactionState{}, err
	}

	return s.repository.DeleteReaction(ctx, userID, postID)
}

func (s *ContentService) requireDemoUser(ctx context.Context, userID int64) error {
	exists, err := s.repository.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrDemoUserNotFound
	}

	return nil
}

func (s *ContentService) requirePost(ctx context.Context, postID int64) error {
	exists, err := s.repository.PostExists(ctx, postID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	return nil
}
