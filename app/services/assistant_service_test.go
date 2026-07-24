package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"goravel/app/repositories"
)

const (
	assistantServiceTestUserID           = "00000000-0000-4000-8000-000000000001"
	assistantServiceTestUnknownUserID    = "00000000-0000-4000-8000-000000000099"
	assistantServiceTestLandscapeTopicID = "10000000-0000-4000-8000-000000000001"
	assistantServiceTestPeaceTopicID     = "10000000-0000-4000-8000-000000000004"
)

type assistantRepositoryFake struct {
	userExists       bool
	userExistsErr    error
	resolveTopic     repositories.AssistantTopic
	resolveFound     bool
	resolveErr       error
	count            int64
	countErr         error
	resolveTopicFunc func(context.Context, string) (repositories.AssistantTopic, bool, error)
	resolvedValues   []string
	countedTopicIDs  []string
}

func (f *assistantRepositoryFake) UserExists(context.Context, string) (bool, error) {
	return f.userExists, f.userExistsErr
}

func (f *assistantRepositoryFake) ResolveTopic(ctx context.Context, normalized string) (repositories.AssistantTopic, bool, error) {
	f.resolvedValues = append(f.resolvedValues, normalized)
	if f.resolveTopicFunc != nil {
		return f.resolveTopicFunc(ctx, normalized)
	}
	return f.resolveTopic, f.resolveFound, f.resolveErr
}

func (f *assistantRepositoryFake) CountPublishedPostsByTopic(_ context.Context, topicID string) (int64, error) {
	f.countedTopicIDs = append(f.countedTopicIDs, topicID)
	return f.count, f.countErr
}

type topicExtractorFake struct {
	candidate string
	err       error
	calls     int
	inputs    []string
}

func (f *topicExtractorFake) Extract(_ context.Context, input string) (string, error) {
	f.calls++
	f.inputs = append(f.inputs, input)
	return f.candidate, f.err
}

func TestAssistantServiceAnswersLocallyUsingTopicAlias(t *testing.T) {
	t.Parallel()

	repository := &assistantRepositoryFake{
		userExists: true,
		resolveTopic: repositories.AssistantTopic{
			ID:      assistantServiceTestLandscapeTopicID,
			Slug:    "phong-canh",
			Name:    "Phong cảnh",
			Aliases: []string{"cảnh vật", "landscape"},
		},
		resolveFound: true,
		count:        8,
	}
	service := NewAssistantService(repository, nil)

	response, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		"Có bao nhiêu bài nói về chủ đề cảnh vật?",
	)

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if response.Status != AssistantStatusAnswered ||
		response.Intent != AssistantIntentCountPostsByTopic ||
		response.Provider != AssistantProviderLocal {
		t.Fatalf("Ask() response metadata = %#v", response)
	}
	if response.Answer != "Hiện có 8 bài viết về chủ đề “Phong cảnh”." {
		t.Fatalf("Ask() answer = %q", response.Answer)
	}
	if response.Result == nil ||
		response.Result.Count != 8 ||
		response.Result.Topic.ID != assistantServiceTestLandscapeTopicID {
		t.Fatalf("Ask() result = %#v", response.Result)
	}
	if len(repository.resolvedValues) != 1 || repository.resolvedValues[0] != "canh vat" {
		t.Fatalf("ResolveTopic() inputs = %#v", repository.resolvedValues)
	}
	if len(repository.countedTopicIDs) != 1 ||
		repository.countedTopicIDs[0] != assistantServiceTestLandscapeTopicID {
		t.Fatalf("CountPublishedPostsByTopic() inputs = %#v", repository.countedTopicIDs)
	}
}

func TestAssistantServiceRequestsClarificationForUnclearQuestion(t *testing.T) {
	t.Parallel()

	repository := &assistantRepositoryFake{userExists: true}
	extractor := &topicExtractorFake{candidate: "phong cảnh"}
	service := NewAssistantService(repository, extractor)

	response, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		"Xin chào, hôm nay bạn khỏe không?",
	)

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if response.Status != AssistantStatusNeedsClarification ||
		response.Intent != AssistantIntentUnknown ||
		response.Provider != AssistantProviderLocal {
		t.Fatalf("Ask() response = %#v", response)
	}
	if response.Answer != "Bạn muốn đếm bài viết về chủ đề nào?" {
		t.Fatalf("Ask() answer = %q", response.Answer)
	}
	if response.Result != nil {
		t.Fatalf("Ask() result = %#v, want nil", response.Result)
	}
	if len(repository.resolvedValues) != 0 || len(repository.countedTopicIDs) != 0 {
		t.Fatalf("repository unexpectedly called: resolve=%#v count=%#v", repository.resolvedValues, repository.countedTopicIDs)
	}
	if extractor.calls != 0 {
		t.Fatalf("Extract() calls = %d, want 0 for a non-count question", extractor.calls)
	}
}

func TestAssistantServiceRequestsClarificationForUnknownTopic(t *testing.T) {
	t.Parallel()

	repository := &assistantRepositoryFake{userExists: true}
	service := NewAssistantService(repository, nil)

	response, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		"Đếm bài về chủ đề không tồn tại",
	)

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if response.Status != AssistantStatusNeedsClarification || response.Result != nil {
		t.Fatalf("Ask() response = %#v", response)
	}
}

func TestAssistantServiceUsesOpenAIExtractorWhenItResolvesAnAllowedTopic(t *testing.T) {
	t.Parallel()

	repository := &assistantRepositoryFake{
		userExists: true,
		resolveTopic: repositories.AssistantTopic{
			ID:   assistantServiceTestPeaceTopicID,
			Slug: "hoa-binh",
			Name: "Hòa bình",
		},
		resolveFound: true,
		count:        4,
	}
	extractor := &topicExtractorFake{candidate: "  “Hòa bình”  "}
	service := NewAssistantService(repository, extractor)

	response, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		"Có bao nhiêu bài về chủ đề “Hòa bình”? email hoc-sinh@example.com",
	)

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if response.Provider != AssistantProviderOpenAI {
		t.Fatalf("Ask() provider = %q, want OPENAI", response.Provider)
	}
	if len(repository.resolvedValues) != 2 ||
		repository.resolvedValues[0] != "hoa binh" ||
		repository.resolvedValues[1] != "hoa binh" {
		t.Fatalf("ResolveTopic() inputs = %#v", repository.resolvedValues)
	}
	if extractor.calls != 1 ||
		len(extractor.inputs) != 1 ||
		extractor.inputs[0] != "Hòa bình" {
		t.Fatalf("Extract() inputs = %#v, want only canonical allowlisted topic name", extractor.inputs)
	}
}

func TestAssistantServiceFallsBackToLocalParserOnOpenAIError(t *testing.T) {
	t.Parallel()

	repository := &assistantRepositoryFake{
		userExists: true,
		resolveTopic: repositories.AssistantTopic{
			ID:   assistantServiceTestLandscapeTopicID,
			Slug: "phong-canh",
			Name: "Phong cảnh",
		},
		resolveFound: true,
		count:        2,
	}
	extractor := &topicExtractorFake{err: errors.New("provider unavailable")}
	service := NewAssistantService(repository, extractor)

	response, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		"Có bao nhiêu bài về phong cảnh?",
	)

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if response.Provider != AssistantProviderLocal {
		t.Fatalf("Ask() provider = %q, want LOCAL", response.Provider)
	}
	if len(repository.resolvedValues) != 1 || repository.resolvedValues[0] != "phong canh" {
		t.Fatalf("ResolveTopic() inputs = %#v", repository.resolvedValues)
	}
}

func TestAssistantServiceKeepsLocalTopicWhenOpenAIReturnsAnotherAllowedTopic(t *testing.T) {
	t.Parallel()

	repository := &assistantRepositoryFake{
		userExists: true,
		resolveTopic: repositories.AssistantTopic{
			ID:   assistantServiceTestLandscapeTopicID,
			Slug: "phong-canh",
			Name: "Phong cảnh",
		},
		count: 6,
	}
	extractor := &topicExtractorFake{candidate: "Hòa bình"}
	service := NewAssistantService(repository, extractor)

	resolveCalls := 0
	repository.resolveTopicFunc = func(context.Context, string) (repositories.AssistantTopic, bool, error) {
		resolveCalls++
		if resolveCalls == 1 {
			return repositories.AssistantTopic{
				ID:   assistantServiceTestLandscapeTopicID,
				Slug: "phong-canh",
				Name: "Phong cảnh",
			}, true, nil
		}
		return repositories.AssistantTopic{
			ID:   assistantServiceTestPeaceTopicID,
			Slug: "hoa-binh",
			Name: "Hòa bình",
		}, true, nil
	}

	response, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		"Có bao nhiêu bài về phong cảnh?",
	)

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if response.Provider != AssistantProviderLocal ||
		response.Result == nil ||
		response.Result.Count != 6 ||
		response.Result.Topic.ID != assistantServiceTestLandscapeTopicID {
		t.Fatalf("Ask() response = %#v", response)
	}
	if len(repository.resolvedValues) != 2 ||
		repository.resolvedValues[0] != "phong canh" ||
		repository.resolvedValues[1] != "hoa binh" {
		t.Fatalf("ResolveTopic() inputs = %#v", repository.resolvedValues)
	}
}

func TestAssistantServiceRejectsUnknownDemoUser(t *testing.T) {
	t.Parallel()

	repository := &assistantRepositoryFake{userExists: false}
	service := NewAssistantService(repository, nil)

	_, err := service.Ask(
		context.Background(),
		assistantServiceTestUnknownUserID,
		"Đếm bài về phong cảnh",
	)

	if !errors.Is(err, ErrDemoUserRequired) {
		t.Fatalf("Ask() error = %v, want ErrDemoUserRequired", err)
	}
}

func TestAssistantServicePropagatesCountFailure(t *testing.T) {
	t.Parallel()

	countFailure := errors.New("count failed")
	repository := &assistantRepositoryFake{
		userExists: true,
		resolveTopic: repositories.AssistantTopic{
			ID:   assistantServiceTestLandscapeTopicID,
			Slug: "phong-canh",
			Name: "Phong cảnh",
		},
		resolveFound: true,
		countErr:     countFailure,
	}
	service := NewAssistantService(repository, nil)

	_, err := service.Ask(
		context.Background(),
		assistantServiceTestUserID,
		"Đếm bài về phong cảnh",
	)

	if !errors.Is(err, countFailure) {
		t.Fatalf("Ask() error = %v, want count failure", err)
	}
}

func TestAssistantResponseJSONMatchesOpenAPIShape(t *testing.T) {
	t.Parallel()

	answered, err := json.Marshal(AssistantResponse{
		Status:   AssistantStatusAnswered,
		Intent:   AssistantIntentCountPostsByTopic,
		Answer:   "Hiện có 8 bài viết về chủ đề “Phong cảnh”.",
		Provider: AssistantProviderLocal,
		Result: &AssistantCountResult{
			Count: 8,
			Topic: AssistantTopic{
				ID:      assistantServiceTestLandscapeTopicID,
				Slug:    "phong-canh",
				Name:    "Phong cảnh",
				Aliases: []string{"cảnh vật"},
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal(answered) error = %v", err)
	}

	wantAnswered := `{"status":"ANSWERED","intent":"COUNT_POSTS_BY_TOPIC","answer":"Hiện có 8 bài viết về chủ đề “Phong cảnh”.","provider":"LOCAL","result":{"count":8,"topic":{"id":"10000000-0000-4000-8000-000000000001","slug":"phong-canh","name":"Phong cảnh","aliases":["cảnh vật"]}}}`
	if string(answered) != wantAnswered {
		t.Fatalf("answered JSON = %s, want %s", answered, wantAnswered)
	}

	clarification, err := json.Marshal(clarificationResponse(AssistantProviderLocal))
	if err != nil {
		t.Fatalf("json.Marshal(clarification) error = %v", err)
	}

	wantClarification := `{"status":"NEEDS_CLARIFICATION","intent":"UNKNOWN","answer":"Bạn muốn đếm bài viết về chủ đề nào?","provider":"LOCAL"}`
	if string(clarification) != wantClarification {
		t.Fatalf("clarification JSON = %s, want %s", clarification, wantClarification)
	}
}
