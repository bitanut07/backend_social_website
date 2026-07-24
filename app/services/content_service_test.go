package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"goravel/app/repositories"
)

const (
	contentServiceUserID        = "00000000-0000-4000-8000-000000000001"
	contentServiceOtherUserID   = "00000000-0000-4000-8000-000000000003"
	contentServiceUnknownUserID = "00000000-0000-4000-8000-000000000404"
	contentServiceTopicID       = "10000000-0000-4000-8000-000000000001"
	contentServiceMissingTopic1 = "10000000-0000-4000-8000-000000000077"
	contentServiceMissingTopic2 = "10000000-0000-4000-8000-000000000088"
	contentServicePostID        = "20000000-0000-4000-8000-000000000001"
	contentServiceOtherPostID   = "20000000-0000-4000-8000-000000000002"
)

type contentListRequest struct {
	page     int
	pageSize int
}

type contentPostListRequest struct {
	viewerID string
	page     int
	pageSize int
	topicID  *string
}

type contentCreateRequest struct {
	userID string
	input  repositories.CreatePostInput
}

type contentReactionRequest struct {
	userID string
	postID string
}

type fakeContentRepository struct {
	usersByID  map[string]bool
	topicsByID map[string]bool
	postsByID  map[string]bool

	userExistsErr     error
	topicExistsErr    error
	missingTopicsErr  error
	postExistsErr     error
	listUsersErr      error
	listTopicsErr     error
	listPostsErr      error
	createPostErr     error
	putReactionErr    error
	deleteReactionErr error

	users               []repositories.User
	usersTotal          int64
	topics              []repositories.Topic
	topicsTotal         int64
	posts               []repositories.Post
	postsTotal          int64
	createdPost         repositories.Post
	missingTopicIDs     []string
	putReactionState    repositories.ReactionState
	deleteReactionState repositories.ReactionState

	userExistenceChecks  []string
	topicExistenceChecks []string
	postExistenceChecks  []string
	missingTopicChecks   [][]string
	listUserRequests     []contentListRequest
	listTopicRequests    []contentListRequest
	listPostRequests     []contentPostListRequest
	createRequests       []contentCreateRequest
	putRequests          []contentReactionRequest
	deleteRequests       []contentReactionRequest
}

func (f *fakeContentRepository) ListUsers(
	_ context.Context,
	page int,
	pageSize int,
) ([]repositories.User, int64, error) {
	f.listUserRequests = append(f.listUserRequests, contentListRequest{page: page, pageSize: pageSize})
	return f.users, f.usersTotal, f.listUsersErr
}

func (f *fakeContentRepository) ListTopics(
	_ context.Context,
	page int,
	pageSize int,
) ([]repositories.Topic, int64, error) {
	f.listTopicRequests = append(f.listTopicRequests, contentListRequest{page: page, pageSize: pageSize})
	return f.topics, f.topicsTotal, f.listTopicsErr
}

func (f *fakeContentRepository) UserExists(_ context.Context, userID string) (bool, error) {
	f.userExistenceChecks = append(f.userExistenceChecks, userID)
	if f.userExistsErr != nil {
		return false, f.userExistsErr
	}

	return f.usersByID[userID], nil
}

func (f *fakeContentRepository) TopicExists(_ context.Context, topicID string) (bool, error) {
	f.topicExistenceChecks = append(f.topicExistenceChecks, topicID)
	if f.topicExistsErr != nil {
		return false, f.topicExistsErr
	}

	return f.topicsByID[topicID], nil
}

func (f *fakeContentRepository) MissingTopicIDs(
	_ context.Context,
	topicIDs []string,
) ([]string, error) {
	f.missingTopicChecks = append(f.missingTopicChecks, append([]string(nil), topicIDs...))
	if f.missingTopicsErr != nil {
		return nil, f.missingTopicsErr
	}

	return append([]string(nil), f.missingTopicIDs...), nil
}

func (f *fakeContentRepository) PostExists(_ context.Context, postID string) (bool, error) {
	f.postExistenceChecks = append(f.postExistenceChecks, postID)
	if f.postExistsErr != nil {
		return false, f.postExistsErr
	}

	return f.postsByID[postID], nil
}

func (f *fakeContentRepository) ListPosts(
	_ context.Context,
	viewerID string,
	page int,
	pageSize int,
	topicID *string,
) ([]repositories.Post, int64, error) {
	var capturedTopicID *string
	if topicID != nil {
		value := *topicID
		capturedTopicID = &value
	}
	f.listPostRequests = append(f.listPostRequests, contentPostListRequest{
		viewerID: viewerID,
		page:     page,
		pageSize: pageSize,
		topicID:  capturedTopicID,
	})

	return f.posts, f.postsTotal, f.listPostsErr
}

func (f *fakeContentRepository) CreatePost(
	_ context.Context,
	userID string,
	input repositories.CreatePostInput,
) (repositories.Post, error) {
	f.createRequests = append(f.createRequests, contentCreateRequest{userID: userID, input: input})
	return f.createdPost, f.createPostErr
}

func (f *fakeContentRepository) PutReaction(
	_ context.Context,
	userID string,
	postID string,
) (repositories.ReactionState, error) {
	f.putRequests = append(f.putRequests, contentReactionRequest{userID: userID, postID: postID})
	return f.putReactionState, f.putReactionErr
}

func (f *fakeContentRepository) DeleteReaction(
	_ context.Context,
	userID string,
	postID string,
) (repositories.ReactionState, error) {
	f.deleteRequests = append(f.deleteRequests, contentReactionRequest{userID: userID, postID: postID})
	return f.deleteReactionState, f.deleteReactionErr
}

func TestContentServiceListsUsersAndTopicsThroughRepository(t *testing.T) {
	t.Parallel()

	t.Run("users", func(t *testing.T) {
		t.Parallel()

		wantUsers := []repositories.User{
			{
				ID:          contentServiceUserID,
				Username:    "linh.ve",
				DisplayName: "Nguyễn Gia Linh",
				Role:        "STUDENT",
			},
			{
				ID:          contentServiceOtherUserID,
				Username:    "co.mai",
				DisplayName: "Cô Mai Anh",
				Role:        "TEACHER",
			},
		}
		repository := &fakeContentRepository{users: wantUsers, usersTotal: 12}
		service := NewContentService(repository)

		users, total, err := service.ListUsers(context.Background(), 2, 5)

		if err != nil {
			t.Fatalf("ListUsers returned unexpected error: %v", err)
		}
		if !reflect.DeepEqual(users, wantUsers) || total != 12 {
			t.Fatalf("ListUsers returned (%#v, %d), want (%#v, 12)", users, total, wantUsers)
		}
		wantRequests := []contentListRequest{{page: 2, pageSize: 5}}
		if !reflect.DeepEqual(repository.listUserRequests, wantRequests) {
			t.Fatalf("ListUsers requests = %#v, want %#v", repository.listUserRequests, wantRequests)
		}
	})

	t.Run("topics", func(t *testing.T) {
		t.Parallel()

		wantTopics := []repositories.Topic{
			{
				ID:      contentServiceTopicID,
				Slug:    "hoa-binh",
				Name:    "Hòa bình",
				Aliases: []string{"peace"},
			},
		}
		repository := &fakeContentRepository{topics: wantTopics, topicsTotal: 7}
		service := NewContentService(repository)

		topics, total, err := service.ListTopics(context.Background(), 3, 4)

		if err != nil {
			t.Fatalf("ListTopics returned unexpected error: %v", err)
		}
		if !reflect.DeepEqual(topics, wantTopics) || total != 7 {
			t.Fatalf("ListTopics returned (%#v, %d), want (%#v, 7)", topics, total, wantTopics)
		}
		wantRequests := []contentListRequest{{page: 3, pageSize: 4}}
		if !reflect.DeepEqual(repository.listTopicRequests, wantRequests) {
			t.Fatalf("ListTopics requests = %#v, want %#v", repository.listTopicRequests, wantRequests)
		}
	})
}

func TestContentServiceListPostsValidatesViewerAndTopicBeforeListing(t *testing.T) {
	t.Parallel()

	topicID := contentServiceTopicID
	wantPosts := []repositories.Post{
		{
			ID:               contentServicePostID,
			Title:            "Thành phố xanh",
			ViewerHasReacted: true,
			CreatedAt:        time.Date(2026, 7, 24, 8, 30, 0, 0, time.FixedZone("ICT", 7*60*60)),
		},
	}

	tests := []struct {
		name                    string
		viewerID                string
		topicID                 *string
		usersByID               map[string]bool
		topicsByID              map[string]bool
		wantErr                 error
		wantTopicChecks         []string
		wantRepositoryListCalls int
	}{
		{
			name:                    "unknown demo user",
			viewerID:                contentServiceUnknownUserID,
			topicID:                 &topicID,
			usersByID:               map[string]bool{contentServiceUserID: true},
			topicsByID:              map[string]bool{topicID: true},
			wantErr:                 ErrDemoUserNotFound,
			wantRepositoryListCalls: 0,
		},
		{
			name:                    "unknown topic filter",
			viewerID:                contentServiceUserID,
			topicID:                 &topicID,
			usersByID:               map[string]bool{contentServiceUserID: true},
			topicsByID:              map[string]bool{},
			wantErr:                 ErrNotFound,
			wantTopicChecks:         []string{topicID},
			wantRepositoryListCalls: 0,
		},
		{
			name:                    "unfiltered feed",
			viewerID:                contentServiceUserID,
			usersByID:               map[string]bool{contentServiceUserID: true},
			topicsByID:              map[string]bool{},
			wantRepositoryListCalls: 1,
		},
		{
			name:                    "existing topic filter",
			viewerID:                contentServiceUserID,
			topicID:                 &topicID,
			usersByID:               map[string]bool{contentServiceUserID: true},
			topicsByID:              map[string]bool{topicID: true},
			wantTopicChecks:         []string{topicID},
			wantRepositoryListCalls: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repository := &fakeContentRepository{
				usersByID:  tt.usersByID,
				topicsByID: tt.topicsByID,
				posts:      wantPosts,
				postsTotal: 31,
			}
			service := NewContentService(repository)

			posts, total, err := service.ListPosts(context.Background(), tt.viewerID, 2, 10, tt.topicID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ListPosts error = %v, want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("ListPosts returned unexpected error: %v", err)
				}
				if !reflect.DeepEqual(posts, wantPosts) || total != 31 {
					t.Fatalf("ListPosts returned (%#v, %d), want (%#v, 31)", posts, total, wantPosts)
				}
			}

			if !reflect.DeepEqual(repository.topicExistenceChecks, tt.wantTopicChecks) {
				t.Fatalf("topic checks = %#v, want %#v", repository.topicExistenceChecks, tt.wantTopicChecks)
			}
			if len(repository.listPostRequests) != tt.wantRepositoryListCalls {
				t.Fatalf("repository ListPosts calls = %d, want %d", len(repository.listPostRequests), tt.wantRepositoryListCalls)
			}
			if tt.wantRepositoryListCalls == 1 {
				request := repository.listPostRequests[0]
				if request.viewerID != tt.viewerID || request.page != 2 || request.pageSize != 10 {
					t.Fatalf("repository ListPosts request = %#v", request)
				}
				if !equalOptionalString(request.topicID, tt.topicID) {
					t.Fatalf("repository topic filter = %#v, want %#v", request.topicID, tt.topicID)
				}
			}
		})
	}
}

func TestContentServiceCreatePostValidatesUserAndAllTopicsBeforeCreating(t *testing.T) {
	t.Parallel()

	input := repositories.CreatePostInput{
		Title:    "Mái trường trong mơ",
		Caption:  "Bài dự thi màu nước.",
		ImageURL: "https://images.example.com/art/school.jpg",
		TopicIDs: []string{
			contentServiceTopicID,
			contentServiceMissingTopic1,
			contentServiceMissingTopic2,
		},
	}
	wantPost := repositories.Post{
		ID:       contentServiceOtherPostID,
		Title:    input.Title,
		Caption:  input.Caption,
		ImageURL: input.ImageURL,
	}

	tests := []struct {
		name                   string
		userID                 string
		usersByID              map[string]bool
		missingTopicIDs        []string
		wantErr                error
		wantMissingTopicChecks int
		wantCreateCalls        int
	}{
		{
			name:            "unknown demo user",
			userID:          contentServiceUnknownUserID,
			usersByID:       map[string]bool{contentServiceUserID: true},
			wantErr:         ErrDemoUserNotFound,
			wantCreateCalls: 0,
		},
		{
			name:                   "one or more topics do not exist",
			userID:                 contentServiceUserID,
			usersByID:              map[string]bool{contentServiceUserID: true},
			missingTopicIDs:        []string{contentServiceMissingTopic1, contentServiceMissingTopic2},
			wantErr:                ErrNotFound,
			wantMissingTopicChecks: 1,
			wantCreateCalls:        0,
		},
		{
			name:                   "valid post",
			userID:                 contentServiceUserID,
			usersByID:              map[string]bool{contentServiceUserID: true},
			wantMissingTopicChecks: 1,
			wantCreateCalls:        1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repository := &fakeContentRepository{
				usersByID:       tt.usersByID,
				missingTopicIDs: tt.missingTopicIDs,
				createdPost:     wantPost,
			}
			service := NewContentService(repository)

			post, err := service.CreatePost(context.Background(), tt.userID, input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("CreatePost error = %v, want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("CreatePost returned unexpected error: %v", err)
				}
				if !reflect.DeepEqual(post, wantPost) {
					t.Fatalf("CreatePost returned %#v, want %#v", post, wantPost)
				}
			}

			if len(repository.missingTopicChecks) != tt.wantMissingTopicChecks {
				t.Fatalf("MissingTopicIDs calls = %d, want %d", len(repository.missingTopicChecks), tt.wantMissingTopicChecks)
			}
			if tt.wantMissingTopicChecks == 1 && !reflect.DeepEqual(repository.missingTopicChecks[0], input.TopicIDs) {
				t.Fatalf("checked topic IDs = %#v, want %#v", repository.missingTopicChecks[0], input.TopicIDs)
			}
			if len(repository.createRequests) != tt.wantCreateCalls {
				t.Fatalf("CreatePost repository calls = %d, want %d", len(repository.createRequests), tt.wantCreateCalls)
			}

			if len(tt.missingTopicIDs) > 0 {
				var missingErr *MissingTopicsError
				if !errors.As(err, &missingErr) {
					t.Fatalf("CreatePost error %T is not *MissingTopicsError", err)
				}
				if !reflect.DeepEqual(missingErr.TopicIDs, tt.missingTopicIDs) {
					t.Fatalf("missing topic IDs = %#v, want %#v", missingErr.TopicIDs, tt.missingTopicIDs)
				}
			}

			if tt.wantCreateCalls == 1 {
				request := repository.createRequests[0]
				if request.userID != tt.userID || !reflect.DeepEqual(request.input, input) {
					t.Fatalf(
						"repository CreatePost request = %#v, want user %q and input %#v",
						request,
						tt.userID,
						input,
					)
				}
			}
		})
	}
}

func TestContentServiceReactionMethodsValidateIdentityAndPost(t *testing.T) {
	t.Parallel()

	operations := []struct {
		name     string
		call     func(*ContentService, context.Context, string, string) (repositories.ReactionState, error)
		requests func(*fakeContentRepository) []contentReactionRequest
	}{
		{
			name: "put",
			call: func(service *ContentService, ctx context.Context, userID, postID string) (repositories.ReactionState, error) {
				return service.PutReaction(ctx, userID, postID)
			},
			requests: func(repository *fakeContentRepository) []contentReactionRequest {
				return repository.putRequests
			},
		},
		{
			name: "delete",
			call: func(service *ContentService, ctx context.Context, userID, postID string) (repositories.ReactionState, error) {
				return service.DeleteReaction(ctx, userID, postID)
			},
			requests: func(repository *fakeContentRepository) []contentReactionRequest {
				return repository.deleteRequests
			},
		},
	}

	for _, operation := range operations {
		operation := operation
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name           string
				usersByID      map[string]bool
				postsByID      map[string]bool
				wantErr        error
				wantPostChecks []string
			}{
				{
					name:      "rejects unknown demo user",
					usersByID: map[string]bool{},
					postsByID: map[string]bool{contentServicePostID: true},
					wantErr:   ErrDemoUserNotFound,
				},
				{
					name:           "rejects unknown post",
					usersByID:      map[string]bool{contentServiceOtherUserID: true},
					postsByID:      map[string]bool{},
					wantErr:        ErrNotFound,
					wantPostChecks: []string{contentServicePostID},
				},
			}

			for _, tt := range tests {
				tt := tt
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					repository := &fakeContentRepository{
						usersByID: tt.usersByID,
						postsByID: tt.postsByID,
					}
					service := NewContentService(repository)

					_, err := operation.call(
						service,
						context.Background(),
						contentServiceOtherUserID,
						contentServicePostID,
					)

					if !errors.Is(err, tt.wantErr) {
						t.Fatalf("%s reaction error = %v, want %v", operation.name, err, tt.wantErr)
					}
					if !reflect.DeepEqual(repository.postExistenceChecks, tt.wantPostChecks) {
						t.Fatalf("post checks = %#v, want %#v", repository.postExistenceChecks, tt.wantPostChecks)
					}
					if requests := operation.requests(repository); len(requests) != 0 {
						t.Fatalf("%s reaction must not reach repository after validation failure: %#v", operation.name, requests)
					}
				})
			}
		})
	}
}

func TestContentServiceReactionMethodsPreserveRepositoryIdempotence(t *testing.T) {
	t.Parallel()

	operations := []struct {
		name      string
		wantState repositories.ReactionState
		configure func(*fakeContentRepository, repositories.ReactionState)
		call      func(*ContentService, context.Context, string, string) (repositories.ReactionState, error)
		requests  func(*fakeContentRepository) []contentReactionRequest
	}{
		{
			name:      "put",
			wantState: repositories.ReactionState{ReactionCount: 6, ViewerHasReacted: true},
			configure: func(repository *fakeContentRepository, state repositories.ReactionState) {
				repository.putReactionState = state
			},
			call: func(service *ContentService, ctx context.Context, userID, postID string) (repositories.ReactionState, error) {
				return service.PutReaction(ctx, userID, postID)
			},
			requests: func(repository *fakeContentRepository) []contentReactionRequest {
				return repository.putRequests
			},
		},
		{
			name:      "delete",
			wantState: repositories.ReactionState{ReactionCount: 5, ViewerHasReacted: false},
			configure: func(repository *fakeContentRepository, state repositories.ReactionState) {
				repository.deleteReactionState = state
			},
			call: func(service *ContentService, ctx context.Context, userID, postID string) (repositories.ReactionState, error) {
				return service.DeleteReaction(ctx, userID, postID)
			},
			requests: func(repository *fakeContentRepository) []contentReactionRequest {
				return repository.deleteRequests
			},
		},
	}

	for _, operation := range operations {
		operation := operation
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()

			repository := &fakeContentRepository{
				usersByID: map[string]bool{contentServiceOtherUserID: true},
				postsByID: map[string]bool{contentServicePostID: true},
			}
			operation.configure(repository, operation.wantState)
			service := NewContentService(repository)

			first, err := operation.call(
				service,
				context.Background(),
				contentServiceOtherUserID,
				contentServicePostID,
			)
			if err != nil {
				t.Fatalf("first %s reaction returned unexpected error: %v", operation.name, err)
			}
			second, err := operation.call(
				service,
				context.Background(),
				contentServiceOtherUserID,
				contentServicePostID,
			)
			if err != nil {
				t.Fatalf("second %s reaction returned unexpected error: %v", operation.name, err)
			}

			if !reflect.DeepEqual(first, operation.wantState) || !reflect.DeepEqual(second, operation.wantState) {
				t.Fatalf("%s states = (%#v, %#v), want stable repository state %#v", operation.name, first, second, operation.wantState)
			}
			wantRequests := []contentReactionRequest{
				{userID: contentServiceOtherUserID, postID: contentServicePostID},
				{userID: contentServiceOtherUserID, postID: contentServicePostID},
			}
			if requests := operation.requests(repository); !reflect.DeepEqual(requests, wantRequests) {
				t.Fatalf("%s requests = %#v, want %#v", operation.name, requests, wantRequests)
			}
		})
	}
}

func TestContentServicePropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()

	repositoryErr := errors.New("content repository unavailable")
	topicID := contentServiceTopicID
	input := repositories.CreatePostInput{TopicIDs: []string{topicID}}

	tests := []struct {
		name string
		run  func(*fakeContentRepository) error
	}{
		{
			name: "list users",
			run: func(repository *fakeContentRepository) error {
				repository.listUsersErr = repositoryErr
				service := NewContentService(repository)
				_, _, err := service.ListUsers(context.Background(), 1, 20)
				return err
			},
		},
		{
			name: "list topics",
			run: func(repository *fakeContentRepository) error {
				repository.listTopicsErr = repositoryErr
				service := NewContentService(repository)
				_, _, err := service.ListTopics(context.Background(), 1, 20)
				return err
			},
		},
		{
			name: "list posts user check",
			run: func(repository *fakeContentRepository) error {
				repository.userExistsErr = repositoryErr
				service := NewContentService(repository)
				_, _, err := service.ListPosts(
					context.Background(),
					contentServiceUserID,
					1,
					10,
					nil,
				)
				return err
			},
		},
		{
			name: "list posts topic check",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[string]bool{contentServiceUserID: true}
				repository.topicExistsErr = repositoryErr
				service := NewContentService(repository)
				_, _, err := service.ListPosts(
					context.Background(),
					contentServiceUserID,
					1,
					10,
					&topicID,
				)
				return err
			},
		},
		{
			name: "list posts query",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[string]bool{contentServiceUserID: true}
				repository.listPostsErr = repositoryErr
				service := NewContentService(repository)
				_, _, err := service.ListPosts(
					context.Background(),
					contentServiceUserID,
					1,
					10,
					nil,
				)
				return err
			},
		},
		{
			name: "create post missing topic query",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[string]bool{contentServiceUserID: true}
				repository.missingTopicsErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.CreatePost(context.Background(), contentServiceUserID, input)
				return err
			},
		},
		{
			name: "create post write",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[string]bool{contentServiceUserID: true}
				repository.createPostErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.CreatePost(context.Background(), contentServiceUserID, input)
				return err
			},
		},
		{
			name: "put reaction post check",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[string]bool{contentServiceUserID: true}
				repository.postExistsErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.PutReaction(
					context.Background(),
					contentServiceUserID,
					contentServicePostID,
				)
				return err
			},
		},
		{
			name: "put reaction write",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[string]bool{contentServiceUserID: true}
				repository.postsByID = map[string]bool{contentServicePostID: true}
				repository.putReactionErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.PutReaction(
					context.Background(),
					contentServiceUserID,
					contentServicePostID,
				)
				return err
			},
		},
		{
			name: "delete reaction write",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[string]bool{contentServiceUserID: true}
				repository.postsByID = map[string]bool{contentServicePostID: true}
				repository.deleteReactionErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.DeleteReaction(
					context.Background(),
					contentServiceUserID,
					contentServicePostID,
				)
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run(&fakeContentRepository{})

			if !errors.Is(err, repositoryErr) {
				t.Fatalf("error = %v, want repository error %v", err, repositoryErr)
			}
		})
	}
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}
