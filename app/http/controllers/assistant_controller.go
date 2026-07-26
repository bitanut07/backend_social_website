package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/http"

	appai "goravel/app/ai"
	"goravel/app/facades"
	"goravel/app/http/responses"
	httpsupport "goravel/app/http/support"
	"goravel/app/repositories"
	"goravel/app/services"
)

const (
	maximumAssistantQuestionLength       = 500
	maximumAssistantAnswerHistoryLength  = 2000
	maximumAssistantHistoryMessages      = 8
	maximumAssistantRequestBytes         = 20 * 1024
	defaultAssistantConversationPageSize = 30
	maximumAssistantConversationPageSize = 100
)

var errMalformedAssistantJSON = errors.New("malformed assistant request JSON")

type assistantQuestionService interface {
	Ask(
		context.Context,
		string,
		string,
		string,
		[]services.AssistantConversationMessage,
	) (services.AssistantResponse, error)
	List(
		context.Context,
		string,
		int,
		int,
	) (services.AssistantConversationListResult, error)
	Get(
		context.Context,
		string,
		string,
	) (services.AssistantConversationDTO, error)
}

type AssistantController struct {
	service                    assistantQuestionService
	reportAssistantUnavailable assistantUnavailableReporter
}

type assistantUnavailableReporter func(http.Context, string, error)

func NewAssistantController() *AssistantController {
	repository := repositories.NewAssistantRepository(facades.Orm())
	extractor := appai.NewConfiguredOpenAITopicExtractor(repository)
	responder := appai.NewConfiguredModelLLMAssistant()
	var topicExtractor services.TopicExtractor
	if extractor != nil {
		topicExtractor = extractor
	}
	service := services.NewAssistantService(repository, topicExtractor)
	if responder != nil {
		service = services.NewAssistantService(
			repository,
			topicExtractor,
			responder,
		)
	}

	historyRepository := repositories.NewAssistantHistoryRepository()
	return newAssistantControllerWithReporter(
		services.NewAssistantHistoryService(historyRepository, service),
		func(ctx http.Context, userID string, err error) {
			facades.Log().
				WithContext(ctx).
				With(map[string]any{
					"user_id": userID,
					"error":   err.Error(),
				}).
				Error("assistant model request failed")
		},
	)
}

func newAssistantController(service assistantQuestionService) *AssistantController {
	return newAssistantControllerWithReporter(service, nil)
}

func newAssistantControllerWithReporter(
	service assistantQuestionService,
	reporter assistantUnavailableReporter,
) *AssistantController {
	return &AssistantController{
		service:                    service,
		reportAssistantUnavailable: reporter,
	}
}

func (c *AssistantController) Ask(ctx http.Context) http.Response {
	userID, err := httpsupport.CurrentUserID(ctx)
	if err != nil {
		return demoUserRequiredResponse(ctx)
	}

	payload, err := decodeAssistantQuestion(ctx)
	if err != nil {
		var typeError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &typeError):
			return assistantValidationResponse(ctx, "Câu hỏi phải là chuỗi văn bản.")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			return responses.Error(
				ctx,
				http.StatusUnprocessableEntity,
				"VALIDATION_ERROR",
				"Dữ liệu không hợp lệ",
				http.Json{},
			)
		default:
			return responses.Error(
				ctx,
				http.StatusBadRequest,
				"BAD_REQUEST",
				"Không thể đọc dữ liệu gửi lên",
				http.Json{},
			)
		}
	}

	question := ""
	if payload.Question != nil {
		question = *payload.Question
	}
	question = strings.TrimSpace(question)
	switch {
	case question == "":
		return assistantValidationResponse(ctx, "Câu hỏi là bắt buộc.")
	case utf8.RuneCountInString(question) > maximumAssistantQuestionLength:
		return assistantValidationResponse(ctx, "Câu hỏi không được vượt quá 500 ký tự.")
	}

	history, historyValidationMessage := validateAssistantHistory(payload.History)
	if historyValidationMessage != "" {
		return assistantFieldValidationResponse(ctx, "history", historyValidationMessage)
	}

	conversationID, validationMessage := validateAssistantConversationID(
		payload.ConversationID,
	)
	if validationMessage != "" {
		return assistantFieldValidationResponse(
			ctx,
			"conversationId",
			validationMessage,
		)
	}

	result, err := c.service.Ask(
		ctx,
		userID,
		conversationID,
		question,
		history,
	)
	if errors.Is(err, services.ErrDemoUserRequired) {
		return demoUserRequiredResponse(ctx)
	}
	if errors.Is(err, services.ErrAssistantConversationNotFound) {
		return assistantConversationNotFoundResponse(ctx)
	}
	if errors.Is(err, services.ErrAssistantUnavailable) {
		if c.reportAssistantUnavailable != nil {
			c.reportAssistantUnavailable(ctx, userID, err)
		}
		return responses.Error(
			ctx,
			http.StatusServiceUnavailable,
			"ASSISTANT_UNAVAILABLE",
			"Trợ lý đang tạm thời chưa kết nối được với mô hình, vui lòng thử lại sau",
			http.Json{},
		)
	}
	if err != nil {
		return responses.Error(
			ctx,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"Đã xảy ra lỗi nội bộ, vui lòng thử lại sau",
			http.Json{},
		)
	}

	return responses.JSON(ctx, http.StatusOK, result)
}

type assistantQuestionPayload struct {
	Question       *string                                 `json:"question"`
	ConversationID *string                                 `json:"conversationId"`
	History        []services.AssistantConversationMessage `json:"history"`
}

func (c *AssistantController) ListConversations(ctx http.Context) http.Response {
	userID, err := httpsupport.CurrentUserID(ctx)
	if err != nil {
		return demoUserRequiredResponse(ctx)
	}

	page, pageSize, failure := contentPagination(
		ctx.Request().Queries(),
		defaultAssistantConversationPageSize,
		maximumAssistantConversationPageSize,
	)
	if failure != nil {
		return failure.respond(ctx)
	}

	result, err := c.service.List(ctx, userID, page, pageSize)
	if errors.Is(err, services.ErrDemoUserRequired) {
		return demoUserRequiredResponse(ctx)
	}
	if err != nil {
		return assistantInternalErrorResponse(ctx)
	}

	return responses.Paginated(
		ctx,
		result.Conversations,
		responses.Page(result.TotalItems, page, pageSize),
	)
}

func (c *AssistantController) ShowConversation(ctx http.Context) http.Response {
	userID, err := httpsupport.CurrentUserID(ctx)
	if err != nil {
		return demoUserRequiredResponse(ctx)
	}
	conversationID, err := httpsupport.ParseResourceID(
		ctx.Request().Route("id"),
	)
	if err != nil {
		return responses.Error(
			ctx,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"ID cuộc trò chuyện không hợp lệ",
			http.Json{},
		)
	}

	result, err := c.service.Get(ctx, userID, conversationID)
	if errors.Is(err, services.ErrDemoUserRequired) {
		return demoUserRequiredResponse(ctx)
	}
	if errors.Is(err, services.ErrAssistantConversationNotFound) {
		return assistantConversationNotFoundResponse(ctx)
	}
	if err != nil {
		return assistantInternalErrorResponse(ctx)
	}
	return responses.Data(ctx, http.StatusOK, result)
}

func decodeAssistantQuestion(ctx http.Context) (assistantQuestionPayload, error) {
	request := ctx.Request().Origin()
	if request == nil || request.Body == nil {
		return assistantQuestionPayload{}, errMalformedAssistantJSON
	}
	if request.ContentLength > maximumAssistantRequestBytes {
		return assistantQuestionPayload{}, errMalformedAssistantJSON
	}

	var payload assistantQuestionPayload
	decoder := json.NewDecoder(io.LimitReader(request.Body, maximumAssistantRequestBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return assistantQuestionPayload{}, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return assistantQuestionPayload{}, errMalformedAssistantJSON
	}

	return payload, nil
}

func validateAssistantHistory(
	history []services.AssistantConversationMessage,
) ([]services.AssistantConversationMessage, string) {
	if len(history) > maximumAssistantHistoryMessages {
		return nil, "Lịch sử hội thoại không được vượt quá 8 tin nhắn."
	}
	if len(history)%2 != 0 {
		return nil, "Lịch sử hội thoại phải gồm các cặp tin nhắn người dùng và trợ lý."
	}

	validated := make([]services.AssistantConversationMessage, 0, len(history))
	for index, message := range history {
		expectedRole := "USER"
		maximumLength := maximumAssistantQuestionLength
		if index%2 == 1 {
			expectedRole = "ASSISTANT"
			maximumLength = maximumAssistantAnswerHistoryLength
		}
		content := strings.TrimSpace(message.Content)
		if message.Role != expectedRole {
			return nil, "Vai trò trong lịch sử hội thoại không đúng thứ tự."
		}
		if content == "" || utf8.RuneCountInString(content) > maximumLength ||
			strings.ContainsRune(content, '\x00') {
			return nil, "Nội dung trong lịch sử hội thoại không hợp lệ."
		}
		validated = append(validated, services.AssistantConversationMessage{
			Role:    message.Role,
			Content: content,
		})
	}

	return validated, ""
}

func validateAssistantConversationID(value *string) (string, string) {
	if value == nil {
		return "", ""
	}
	if strings.TrimSpace(*value) == "" {
		return "", "ID cuộc trò chuyện không được để trống."
	}
	parsed, err := httpsupport.ParseResourceID(*value)
	if err != nil {
		return "", "ID cuộc trò chuyện phải là một UUID hợp lệ."
	}
	return parsed, ""
}

func demoUserRequiredResponse(ctx http.Context) http.Response {
	return responses.Error(
		ctx,
		http.StatusUnauthorized,
		"DEMO_USER_REQUIRED",
		"Vui lòng chọn một tài khoản mẫu",
		http.Json{"header": "X-User-ID"},
	)
}

func assistantValidationResponse(ctx http.Context, message string) http.Response {
	return assistantFieldValidationResponse(ctx, "question", message)
}

func assistantFieldValidationResponse(
	ctx http.Context,
	field string,
	message string,
) http.Response {
	return responses.Error(
		ctx,
		http.StatusUnprocessableEntity,
		"VALIDATION_ERROR",
		"Dữ liệu không hợp lệ",
		http.Json{
			"fields": http.Json{
				field: []string{message},
			},
		},
	)
}

func assistantConversationNotFoundResponse(ctx http.Context) http.Response {
	return responses.Error(
		ctx,
		http.StatusNotFound,
		"NOT_FOUND",
		"Không tìm thấy cuộc trò chuyện",
		http.Json{},
	)
}

func assistantInternalErrorResponse(ctx http.Context) http.Response {
	return responses.Error(
		ctx,
		http.StatusInternalServerError,
		"INTERNAL_ERROR",
		"Đã xảy ra lỗi nội bộ, vui lòng thử lại sau",
		http.Json{},
	)
}
