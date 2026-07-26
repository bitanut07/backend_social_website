package services

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"goravel/app/repositories"
)

const (
	AssistantStatusAnswered           = "ANSWERED"
	AssistantStatusNeedsClarification = "NEEDS_CLARIFICATION"

	AssistantIntentCountPostsByTopic = "COUNT_POSTS_BY_TOPIC"
	AssistantIntentAppServiceHelp    = "APP_SERVICE_HELP"
	AssistantIntentChat              = "CHAT"
	AssistantIntentUnknown           = "UNKNOWN"

	AssistantProviderLocal    = "LOCAL"
	AssistantProviderOpenAI   = "OPENAI"
	AssistantProviderModelLLM = "MODEL_LLM"

	AssistantModelActionAnswer = "ANSWER"
	AssistantModelActionCount  = "COUNT_POSTS_BY_TOPIC"

	maximumAssistantAnswerLength = 2000
	assistantModelRetryTimeout   = 30 * time.Second
)

var (
	ErrDemoUserRequired     = errors.New("demo user is missing or does not exist")
	ErrAssistantUnavailable = errors.New("assistant model is unavailable")
)

type AssistantTopic struct {
	ID      string   `json:"id"`
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
}

type AssistantCountResult struct {
	Count int64          `json:"count"`
	Topic AssistantTopic `json:"topic"`
}

type AssistantResponse struct {
	Status       string                        `json:"status"`
	Intent       string                        `json:"intent"`
	Answer       string                        `json:"answer"`
	Provider     string                        `json:"provider"`
	AppService   AssistantAppService           `json:"appService,omitempty"`
	Result       *AssistantCountResult         `json:"result,omitempty"`
	Conversation *AssistantConversationSummary `json:"conversation,omitempty"`
}

type AssistantDataRepository interface {
	UserExists(context.Context, string) (bool, error)
	ResolveTopic(context.Context, string) (repositories.AssistantTopic, bool, error)
	CountPublishedPostsByTopic(context.Context, string) (int64, error)
}

type TopicExtractor interface {
	Extract(context.Context, string) (string, error)
}

type AssistantModelDecision struct {
	Action string
	Topic  string
	Answer string
}

type AssistantConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AssistantResponder interface {
	Respond(context.Context, string) (AssistantModelDecision, error)
}

type AssistantConversationResponder interface {
	RespondConversation(
		context.Context,
		string,
		[]AssistantConversationMessage,
	) (AssistantModelDecision, error)
}

type AssistantService struct {
	repository AssistantDataRepository
	extractor  TopicExtractor
	responder  AssistantResponder
}

func NewAssistantService(
	repository AssistantDataRepository,
	extractor TopicExtractor,
	responders ...AssistantResponder,
) *AssistantService {
	service := &AssistantService{
		repository: repository,
	}
	if !isNilAssistantDependency(extractor) {
		service.extractor = extractor
	}
	if len(responders) > 0 && !isNilAssistantDependency(responders[0]) {
		service.responder = responders[0]
	}

	return service
}

func isNilAssistantDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func (s *AssistantService) Ask(
	ctx context.Context,
	userID string,
	question string,
) (AssistantResponse, error) {
	return s.AskConversation(ctx, userID, question, nil)
}

func (s *AssistantService) AskConversation(
	ctx context.Context,
	userID string,
	question string,
	history []AssistantConversationMessage,
) (AssistantResponse, error) {
	exists, err := s.repository.UserExists(ctx, userID)
	if err != nil {
		return AssistantResponse{}, err
	}
	if !exists {
		return AssistantResponse{}, ErrDemoUserRequired
	}

	localCandidate := ExtractTopicCandidate(question)
	if localCandidate != "" {
		topic, found, err := s.repository.ResolveTopic(ctx, localCandidate)
		if err != nil {
			return AssistantResponse{}, err
		}
		if found {
			return s.countResolvedTopic(ctx, topic, AssistantProviderLocal, true)
		}
	}

	if appService := DetectAppService(question, history); appService != "" {
		return appServiceHelpResponse(appService), nil
	}

	if s.responder == nil {
		return clarificationResponse(AssistantProviderLocal), nil
	}

	decision, err := s.requestModelDecision(ctx, question, history)
	if err != nil {
		if localCandidate != "" {
			return clarificationResponse(AssistantProviderLocal), nil
		}
		return AssistantResponse{}, fmt.Errorf(
			"%w: %w",
			ErrAssistantUnavailable,
			err,
		)
	}

	switch decision.Action {
	case AssistantModelActionAnswer:
		answer := strings.TrimSpace(decision.Answer)
		if answer == "" ||
			utf8.RuneCountInString(answer) > maximumAssistantAnswerLength ||
			strings.ContainsRune(answer, '\x00') {
			return AssistantResponse{}, ErrAssistantUnavailable
		}

		return AssistantResponse{
			Status:   AssistantStatusAnswered,
			Intent:   AssistantIntentChat,
			Answer:   answer,
			Provider: AssistantProviderModelLLM,
		}, nil
	case AssistantModelActionCount:
		normalized := limitTopic(NormalizeForSearch(decision.Topic))
		if normalized == "" {
			return clarificationResponse(AssistantProviderModelLLM), nil
		}

		topic, found, resolveErr := s.repository.ResolveTopic(ctx, normalized)
		if resolveErr != nil {
			return AssistantResponse{}, resolveErr
		}
		if !found {
			return clarificationResponse(AssistantProviderModelLLM), nil
		}

		return s.countResolvedTopic(ctx, topic, AssistantProviderModelLLM, false)
	default:
		return AssistantResponse{}, ErrAssistantUnavailable
	}
}

func (s *AssistantService) requestModelDecision(
	ctx context.Context,
	question string,
	history []AssistantConversationMessage,
) (AssistantModelDecision, error) {
	respond := func(requestContext context.Context) (AssistantModelDecision, error) {
		if conversationResponder, ok := s.responder.(AssistantConversationResponder); ok {
			return conversationResponder.RespondConversation(
				requestContext,
				question,
				history,
			)
		}

		return s.responder.Respond(requestContext, question)
	}

	decision, err := respond(ctx)
	if err == nil || ctx.Err() != nil || !assistantModelTimedOut(err) {
		return decision, err
	}

	retryContext, cancel := context.WithTimeout(ctx, assistantModelRetryTimeout)
	defer cancel()

	return respond(retryContext)
}

func assistantModelTimedOut(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

func (s *AssistantService) countResolvedTopic(
	ctx context.Context,
	topic repositories.AssistantTopic,
	provider string,
	verifyWithExtractor bool,
) (AssistantResponse, error) {
	if verifyWithExtractor && s.extractor != nil {
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
