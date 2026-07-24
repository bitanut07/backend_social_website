package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"goravel/app/repositories"
)

type contentListRequest struct {
	page     int
	pageSize int
}

type contentPostListRequest struct {
	viewerID int64
	page     int
	pageSize int
	topicID  *int64
}

type contentCreateRequest struct {
	userID int64
	input  repositories.CreatePostInput
}

type contentReactionRequest struct {
	userID int64
	postID int64
}

type fakeContentRepository struct {
	usersByID  map[int64]bool
	topicsByID map[int64]bool
	postsByID  map[int64]bool

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
	missingTopicIDs     []int64
	putReactionState    repositories.ReactionState
	deleteReactionState repositories.ReactionState

	userExistenceChecks  []int64
	topicExistenceChecks []int64
	postExistenceChecks  []int64
	missingTopicChecks   [][]int64
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

func (f *fakeContentRepository) UserExists(_ context.Context, userID int64) (bool, error) {
	f.userExistenceChecks = append(f.userExistenceChecks, userID)
	if f.userExistsErr != nil {
		return false, f.userExistsErr
	}

	return f.usersByID[userID], nil
}

func (f *fakeContentRepository) TopicExists(_ context.Context, topicID int64) (bool, error) {
	f.topicExistenceChecks = append(f.topicExistenceChecks, topicID)
	if f.topicExistsErr != nil {
		return false, f.topicExistsErr
	}

	return f.topicsByID[topicID], nil
}

func (f *fakeContentRepository) MissingTopicIDs(
	_ context.Context,
	topicIDs []int64,
) ([]int64, error) {
	f.missingTopicChecks = append(f.missingTopicChecks, append([]int64(nil), topicIDs...))
	if f.missingTopicsErr != nil {
		return nil, f.missingTopicsErr
	}

	return append([]int64(nil), f.missingTopicIDs...), nil
}

func (f *fakeContentRepository) PostExists(_ context.Context, postID int64) (bool, error) {
	f.postExistenceChecks = append(f.postExistenceChecks, postID)
	if f.postExistsErr != nil {
		return false, f.postExistsErr
	}

	return f.postsByID[postID], nil
}

func (f *fakeContentRepository) ListPosts(
	_ context.Context,
	viewerID int64,
	page int,
	pageSize int,
	topicID *int64,
) ([]repositories.Post, int64, error) {
	var capturedTopicID *int64
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
	userID int64,
	input repositories.CreatePostInput,
) (repositories.Post, error) {
	f.createRequests = append(f.createRequests, contentCreateRequest{userID: userID, input: input})
	return f.createdPost, f.createPostErr
}

func (f *fakeContentRepository) PutReaction(
	_ context.Context,
	userID int64,
	postID int64,
) (repositories.ReactionState, error) {
	f.putRequests = append(f.putRequests, contentReactionRequest{userID: userID, postID: postID})
	return f.putReactionState, f.putReactionErr
}

func (f *fakeContentRepository) DeleteReaction(
	_ context.Context,
	userID int64,
	postID int64,
) (repositories.ReactionState, error) {
	f.deleteRequests = append(f.deleteRequests, contentReactionRequest{userID: userID, postID: postID})
	return f.deleteReactionState, f.deleteReactionErr
}

func TestContentServiceListsUsersAndTopicsThroughRepository(t *testing.T) {
	t.Parallel()

	t.Run("users", func(t *testing.T) {
		t.Parallel()

		wantUsers := []repositories.User{
			{ID: 1, Username: "linh.ve", DisplayName: "Nguyễn Gia Linh", Role: "STUDENT"},
			{ID: 2, Username: "co.mai", DisplayName: "Cô Mai Anh", Role: "TEACHER"},
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
			{ID: 3, Slug: "hoa-binh", Name: "Hòa bình", Aliases: []string{"peace"}},
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

	topicID := int64(9)
	wantPosts := []repositories.Post{
		{
			ID:               21,
			Title:            "Thành phố xanh",
			ViewerHasReacted: true,
			CreatedAt:        time.Date(2026, 7, 24, 8, 30, 0, 0, time.FixedZone("ICT", 7*60*60)),
		},
	}

	tests := []struct {
		name                    string
		viewerID                int64
		topicID                 *int64
		usersByID               map[int64]bool
		topicsByID              map[int64]bool
		wantErr                 error
		wantTopicChecks         []int64
		wantRepositoryListCalls int
	}{
		{
			name:                    "unknown demo user",
			viewerID:                404,
			topicID:                 &topicID,
			usersByID:               map[int64]bool{1: true},
			topicsByID:              map[int64]bool{topicID: true},
			wantErr:                 ErrDemoUserNotFound,
			wantRepositoryListCalls: 0,
		},
		{
			name:                    "unknown topic filter",
			viewerID:                1,
			topicID:                 &topicID,
			usersByID:               map[int64]bool{1: true},
			topicsByID:              map[int64]bool{},
			wantErr:                 ErrNotFound,
			wantTopicChecks:         []int64{topicID},
			wantRepositoryListCalls: 0,
		},
		{
			name:                    "unfiltered feed",
			viewerID:                1,
			usersByID:               map[int64]bool{1: true},
			topicsByID:              map[int64]bool{},
			wantRepositoryListCalls: 1,
		},
		{
			name:                    "existing topic filter",
			viewerID:                1,
			topicID:                 &topicID,
			usersByID:               map[int64]bool{1: true},
			topicsByID:              map[int64]bool{topicID: true},
			wantTopicChecks:         []int64{topicID},
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
				if !equalOptionalInt64(request.topicID, tt.topicID) {
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
		TopicIDs: []int64{2, 77, 88},
	}
	wantPost := repositories.Post{ID: 42, Title: input.Title, Caption: input.Caption, ImageURL: input.ImageURL}

	tests := []struct {
		name                   string
		userID                 int64
		usersByID              map[int64]bool
		missingTopicIDs        []int64
		wantErr                error
		wantMissingTopicChecks int
		wantCreateCalls        int
	}{
		{
			name:            "unknown demo user",
			userID:          404,
			usersByID:       map[int64]bool{1: true},
			wantErr:         ErrDemoUserNotFound,
			wantCreateCalls: 0,
		},
		{
			name:                   "one or more topics do not exist",
			userID:                 1,
			usersByID:              map[int64]bool{1: true},
			missingTopicIDs:        []int64{77, 88},
			wantErr:                ErrNotFound,
			wantMissingTopicChecks: 1,
			wantCreateCalls:        0,
		},
		{
			name:                   "valid post",
			userID:                 1,
			usersByID:              map[int64]bool{1: true},
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
					t.Fatalf("repository CreatePost request = %#v, want user %d and input %#v", request, tt.userID, input)
				}
			}
		})
	}
}

func TestContentServiceReactionMethodsValidateIdentityAndPost(t *testing.T) {
	t.Parallel()

	operations := []struct {
		name     string
		call     func(*ContentService, context.Context, int64, int64) (repositories.ReactionState, error)
		requests func(*fakeContentRepository) []contentReactionRequest
	}{
		{
			name: "put",
			call: func(service *ContentService, ctx context.Context, userID, postID int64) (repositories.ReactionState, error) {
				return service.PutReaction(ctx, userID, postID)
			},
			requests: func(repository *fakeContentRepository) []contentReactionRequest {
				return repository.putRequests
			},
		},
		{
			name: "delete",
			call: func(service *ContentService, ctx context.Context, userID, postID int64) (repositories.ReactionState, error) {
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
				usersByID      map[int64]bool
				postsByID      map[int64]bool
				wantErr        error
				wantPostChecks []int64
			}{
				{
					name:      "rejects unknown demo user",
					usersByID: map[int64]bool{},
					postsByID: map[int64]bool{25: true},
					wantErr:   ErrDemoUserNotFound,
				},
				{
					name:           "rejects unknown post",
					usersByID:      map[int64]bool{3: true},
					postsByID:      map[int64]bool{},
					wantErr:        ErrNotFound,
					wantPostChecks: []int64{25},
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

					_, err := operation.call(service, context.Background(), 3, 25)

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
		call      func(*ContentService, context.Context, int64, int64) (repositories.ReactionState, error)
		requests  func(*fakeContentRepository) []contentReactionRequest
	}{
		{
			name:      "put",
			wantState: repositories.ReactionState{ReactionCount: 6, ViewerHasReacted: true},
			configure: func(repository *fakeContentRepository, state repositories.ReactionState) {
				repository.putReactionState = state
			},
			call: func(service *ContentService, ctx context.Context, userID, postID int64) (repositories.ReactionState, error) {
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
			call: func(service *ContentService, ctx context.Context, userID, postID int64) (repositories.ReactionState, error) {
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
				usersByID: map[int64]bool{3: true},
				postsByID: map[int64]bool{25: true},
			}
			operation.configure(repository, operation.wantState)
			service := NewContentService(repository)

			first, err := operation.call(service, context.Background(), 3, 25)
			if err != nil {
				t.Fatalf("first %s reaction returned unexpected error: %v", operation.name, err)
			}
			second, err := operation.call(service, context.Background(), 3, 25)
			if err != nil {
				t.Fatalf("second %s reaction returned unexpected error: %v", operation.name, err)
			}

			if !reflect.DeepEqual(first, operation.wantState) || !reflect.DeepEqual(second, operation.wantState) {
				t.Fatalf("%s states = (%#v, %#v), want stable repository state %#v", operation.name, first, second, operation.wantState)
			}
			wantRequests := []contentReactionRequest{
				{userID: 3, postID: 25},
				{userID: 3, postID: 25},
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
	topicID := int64(9)
	input := repositories.CreatePostInput{TopicIDs: []int64{topicID}}

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
				_, _, err := service.ListPosts(context.Background(), 1, 1, 10, nil)
				return err
			},
		},
		{
			name: "list posts topic check",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[int64]bool{1: true}
				repository.topicExistsErr = repositoryErr
				service := NewContentService(repository)
				_, _, err := service.ListPosts(context.Background(), 1, 1, 10, &topicID)
				return err
			},
		},
		{
			name: "list posts query",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[int64]bool{1: true}
				repository.listPostsErr = repositoryErr
				service := NewContentService(repository)
				_, _, err := service.ListPosts(context.Background(), 1, 1, 10, nil)
				return err
			},
		},
		{
			name: "create post missing topic query",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[int64]bool{1: true}
				repository.missingTopicsErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.CreatePost(context.Background(), 1, input)
				return err
			},
		},
		{
			name: "create post write",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[int64]bool{1: true}
				repository.createPostErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.CreatePost(context.Background(), 1, input)
				return err
			},
		},
		{
			name: "put reaction post check",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[int64]bool{1: true}
				repository.postExistsErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.PutReaction(context.Background(), 1, 5)
				return err
			},
		},
		{
			name: "put reaction write",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[int64]bool{1: true}
				repository.postsByID = map[int64]bool{5: true}
				repository.putReactionErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.PutReaction(context.Background(), 1, 5)
				return err
			},
		},
		{
			name: "delete reaction write",
			run: func(repository *fakeContentRepository) error {
				repository.usersByID = map[int64]bool{1: true}
				repository.postsByID = map[int64]bool{5: true}
				repository.deleteReactionErr = repositoryErr
				service := NewContentService(repository)
				_, err := service.DeleteReaction(context.Background(), 1, 5)
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

func equalOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}
