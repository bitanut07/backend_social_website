package services

import (
	"context"
	"errors"
	"fmt"

	"goravel/app/repositories"
)

const (
	AssistantStatusAnswered           = "ANSWERED"
	AssistantStatusNeedsClarification = "NEEDS_CLARIFICATION"

	AssistantIntentCountPostsByTopic = "COUNT_POSTS_BY_TOPIC"
	AssistantIntentUnknown           = "UNKNOWN"

	AssistantProviderLocal  = "LOCAL"
	AssistantProviderOpenAI = "OPENAI"
)

var ErrDemoUserRequired = errors.New("demo user is missing or does not exist")

type AssistantTopic struct {
	ID      uint64   `json:"id"`
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
}

type AssistantCountResult struct {
	Count int64          `json:"count"`
	Topic AssistantTopic `json:"topic"`
}

type AssistantResponse struct {
	Status   string                `json:"status"`
	Intent   string                `json:"intent"`
	Answer   string                `json:"answer"`
	Provider string                `json:"provider"`
	Result   *AssistantCountResult `json:"result,omitempty"`
}

type AssistantDataRepository interface {
	UserExists(context.Context, uint64) (bool, error)
	ResolveTopic(context.Context, string) (repositories.AssistantTopic, bool, error)
	CountPublishedPostsByTopic(context.Context, uint64) (int64, error)
}

type TopicExtractor interface {
	Extract(context.Context, string) (string, error)
}

type AssistantService struct {
	repository AssistantDataRepository
	extractor  TopicExtractor
}

func NewAssistantService(
	repository AssistantDataRepository,
	extractor TopicExtractor,
) *AssistantService {
	return &AssistantService{
		repository: repository,
		extractor:  extractor,
	}
}

func (s *AssistantService) Ask(
	ctx context.Context,
	userID uint64,
	question string,
) (AssistantResponse, error) {
	exists, err := s.repository.UserExists(ctx, userID)
	if err != nil {
		return AssistantResponse{}, err
	}
	if !exists {
		return AssistantResponse{}, ErrDemoUserRequired
	}

	localCandidate := ExtractTopicCandidate(question)
	if localCandidate == "" {
		return clarificationResponse(AssistantProviderLocal), nil
	}

	topic, found, err := s.repository.ResolveTopic(ctx, localCandidate)
	if err != nil {
		return AssistantResponse{}, err
	}
	if !found {
		return clarificationResponse(AssistantProviderLocal), nil
	}

	provider := AssistantProviderLocal

	if s.extractor != nil {
		extracted, extractionErr := s.extractor.Extract(ctx, topic.Name)
		if extractionErr == nil {
			normalized := limitTopic(NormalizeForSearch(extracted))
			if normalized != "" {
				verifiedTopic, verified, resolveErr := s.repository.ResolveTopic(ctx, normalized)
				if resolveErr != nil {
					return AssistantResponse{}, resolveErr
				}
				if verified && verifiedTopic.ID == topic.ID {
					provider = AssistantProviderOpenAI
				}
			}
		}
	}

	count, err := s.repository.CountPublishedPostsByTopic(ctx, topic.ID)
	if err != nil {
		return AssistantResponse{}, err
	}

	responseTopic := AssistantTopic{
		ID:      topic.ID,
		Slug:    topic.Slug,
		Name:    topic.Name,
		Aliases: topic.Aliases,
	}

	return AssistantResponse{
		Status:   AssistantStatusAnswered,
		Intent:   AssistantIntentCountPostsByTopic,
		Answer:   fmt.Sprintf("Hiện có %d bài viết về chủ đề “%s”.", count, topic.Name),
		Provider: provider,
		Result: &AssistantCountResult{
			Count: count,
			Topic: responseTopic,
		},
	}, nil
}

func clarificationResponse(provider string) AssistantResponse {
	return AssistantResponse{
		Status:   AssistantStatusNeedsClarification,
		Intent:   AssistantIntentUnknown,
		Answer:   "Bạn muốn đếm bài viết về chủ đề nào?",
		Provider: provider,
	}
}
