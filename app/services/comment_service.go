package services

import (
	"context"

	"goravel/app/repositories"
)

type CommentRepository interface {
	UserExists(ctx context.Context, userID string) (bool, error)
	ListByPost(
		ctx context.Context,
		viewerID string,
		postID string,
		page int,
		pageSize int,
	) ([]repositories.Comment, int64, error)
	Create(
		ctx context.Context,
		userID string,
		postID string,
		body string,
	) (repositories.Comment, error)
	Delete(
		ctx context.Context,
		userID string,
		postID string,
		commentID string,
	) error
}

type CommentService struct {
	repository CommentRepository
}

func NewCommentService(repository CommentRepository) *CommentService {
	return &CommentService{repository: repository}
}

func (s *CommentService) List(
	ctx context.Context,
	currentUserID string,
	postID string,
	page int,
	pageSize int,
) ([]repositories.Comment, int64, error) {
	if err := s.requireCurrentUser(ctx, currentUserID); err != nil {
		return nil, 0, err
	}

	return s.repository.ListByPost(ctx, currentUserID, postID, page, pageSize)
}

func (s *CommentService) Create(
	ctx context.Context,
	currentUserID string,
	postID string,
	body string,
) (repositories.Comment, error) {
	if err := s.requireCurrentUser(ctx, currentUserID); err != nil {
		return repositories.Comment{}, err
	}

	return s.repository.Create(ctx, currentUserID, postID, body)
}

func (s *CommentService) Delete(
	ctx context.Context,
	currentUserID string,
	postID string,
	commentID string,
) error {
	if err := s.requireCurrentUser(ctx, currentUserID); err != nil {
		return err
	}

	return s.repository.Delete(ctx, currentUserID, postID, commentID)
}

func (s *CommentService) requireCurrentUser(
	ctx context.Context,
	userID string,
) error {
	exists, err := s.repository.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrDemoUserNotFound
	}

	return nil
}
