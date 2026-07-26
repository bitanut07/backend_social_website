package services

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"goravel/app/repositories"
)

const (
	commentServiceUserID    = "00000000-0000-4000-8000-000000000001"
	commentServicePostID    = "20000000-0000-4000-8000-000000000001"
	commentServiceCommentID = "50000000-0000-4000-8000-000000000001"
)

type commentListRequest struct {
	viewerID string
	postID   string
	page     int
	pageSize int
}

type commentCreateRequest struct {
	userID string
	postID string
	body   string
}

type commentDeleteRequest struct {
	userID    string
	postID    string
	commentID string
}

type fakeCommentRepository struct {
	usersByID map[string]bool

	userExistsErr error
	listErr       error
	createErr     error
	deleteErr     error

	comments []repositories.Comment
	total    int64
	created  repositories.Comment

	userChecks     []string
	listRequests   []commentListRequest
	createRequests []commentCreateRequest
	deleteRequests []commentDeleteRequest
}

func (f *fakeCommentRepository) UserExists(
	_ context.Context,
	userID string,
) (bool, error) {
	f.userChecks = append(f.userChecks, userID)
	if f.userExistsErr != nil {
		return false, f.userExistsErr
	}
	return f.usersByID[userID], nil
}

func (f *fakeCommentRepository) ListByPost(
	_ context.Context,
	viewerID string,
	postID string,
	page int,
	pageSize int,
) ([]repositories.Comment, int64, error) {
	f.listRequests = append(f.listRequests, commentListRequest{
		viewerID: viewerID,
		postID:   postID,
		page:     page,
		pageSize: pageSize,
	})
	return f.comments, f.total, f.listErr
}

func (f *fakeCommentRepository) Create(
	_ context.Context,
	userID string,
	postID string,
	body string,
) (repositories.Comment, error) {
	f.createRequests = append(f.createRequests, commentCreateRequest{
		userID: userID,
		postID: postID,
		body:   body,
	})
	return f.created, f.createErr
}

func (f *fakeCommentRepository) Delete(
	_ context.Context,
	userID string,
	postID string,
	commentID string,
) error {
	f.deleteRequests = append(f.deleteRequests, commentDeleteRequest{
		userID:    userID,
		postID:    postID,
		commentID: commentID,
	})
	return f.deleteErr
}

func TestCommentServiceRequiresKnownDemoUserBeforeAccessingComments(t *testing.T) {
	t.Parallel()

	repository := &fakeCommentRepository{usersByID: map[string]bool{}}
	service := NewCommentService(repository)

	_, _, listErr := service.List(
		context.Background(),
		commentServiceUserID,
		commentServicePostID,
		1,
		20,
	)
	if !errors.Is(listErr, ErrDemoUserNotFound) {
		t.Fatalf("List error = %v, want ErrDemoUserNotFound", listErr)
	}

	_, createErr := service.Create(
		context.Background(),
		commentServiceUserID,
		commentServicePostID,
		"Đẹp quá!",
	)
	if !errors.Is(createErr, ErrDemoUserNotFound) {
		t.Fatalf("Create error = %v, want ErrDemoUserNotFound", createErr)
	}
	deleteErr := service.Delete(
		context.Background(),
		commentServiceUserID,
		commentServicePostID,
		commentServiceCommentID,
	)
	if !errors.Is(deleteErr, ErrDemoUserNotFound) {
		t.Fatalf("Delete error = %v, want ErrDemoUserNotFound", deleteErr)
	}
	if len(repository.listRequests) != 0 ||
		len(repository.createRequests) != 0 ||
		len(repository.deleteRequests) != 0 {
		t.Fatalf(
			"repository calls after unknown user = list %#v, create %#v, delete %#v",
			repository.listRequests,
			repository.createRequests,
			repository.deleteRequests,
		)
	}
}

func TestCommentServiceDelegatesListAndCreateWithCurrentUserAsAuthor(t *testing.T) {
	t.Parallel()

	wantComments := []repositories.Comment{{
		ID:     "50000000-0000-4000-8000-000000000001",
		PostID: commentServicePostID,
		Body:   "Đẹp quá!",
	}}
	repository := &fakeCommentRepository{
		usersByID: map[string]bool{commentServiceUserID: true},
		comments:  wantComments,
		total:     8,
		created:   wantComments[0],
	}
	service := NewCommentService(repository)

	comments, total, err := service.List(
		context.Background(),
		commentServiceUserID,
		commentServicePostID,
		2,
		20,
	)
	if err != nil {
		t.Fatalf("List error = %v", err)
	}
	if !reflect.DeepEqual(comments, wantComments) || total != 8 {
		t.Fatalf("List returned (%#v, %d)", comments, total)
	}

	created, err := service.Create(
		context.Background(),
		commentServiceUserID,
		commentServicePostID,
		"Đẹp quá!",
	)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if !reflect.DeepEqual(created, wantComments[0]) {
		t.Fatalf("Create returned %#v", created)
	}
	if !reflect.DeepEqual(repository.listRequests, []commentListRequest{{
		viewerID: commentServiceUserID,
		postID:   commentServicePostID,
		page:     2,
		pageSize: 20,
	}}) {
		t.Fatalf("list requests = %#v", repository.listRequests)
	}
	if !reflect.DeepEqual(repository.createRequests, []commentCreateRequest{{
		userID: commentServiceUserID,
		postID: commentServicePostID,
		body:   "Đẹp quá!",
	}}) {
		t.Fatalf("create requests = %#v", repository.createRequests)
	}
}

func TestCommentServicePreservesPostAndPolicyErrors(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []error{ErrNotFound, ErrForbidden} {
		sentinel := sentinel
		t.Run(sentinel.Error(), func(t *testing.T) {
			t.Parallel()

			repository := &fakeCommentRepository{
				usersByID: map[string]bool{commentServiceUserID: true},
				createErr: sentinel,
			}
			service := NewCommentService(repository)

			_, err := service.Create(
				context.Background(),
				commentServiceUserID,
				commentServicePostID,
				"Nội dung hợp lệ",
			)

			if !errors.Is(err, sentinel) {
				t.Fatalf("Create error = %v, want %v", err, sentinel)
			}
		})
	}
}

func TestCommentServiceDeletesWithCurrentUserAndPreservesNotFound(t *testing.T) {
	t.Parallel()

	repository := &fakeCommentRepository{
		usersByID: map[string]bool{commentServiceUserID: true},
		deleteErr: ErrNotFound,
	}
	service := NewCommentService(repository)

	err := service.Delete(
		context.Background(),
		commentServiceUserID,
		commentServicePostID,
		commentServiceCommentID,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete error = %v, want ErrNotFound", err)
	}
	if !reflect.DeepEqual(repository.deleteRequests, []commentDeleteRequest{{
		userID:    commentServiceUserID,
		postID:    commentServicePostID,
		commentID: commentServiceCommentID,
	}}) {
		t.Fatalf("delete requests = %#v", repository.deleteRequests)
	}
}
