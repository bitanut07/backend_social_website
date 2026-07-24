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
	maximumAssistantQuestionLength = 500
	maximumAssistantRequestBytes   = 4 * 1024
)

var errMalformedAssistantJSON = errors.New("malformed assistant request JSON")

type assistantQuestionService interface {
	Ask(context.Context, string, string) (services.AssistantResponse, error)
}

type AssistantController struct {
	service assistantQuestionService
}

func NewAssistantController() *AssistantController {
	repository := repositories.NewAssistantRepository(facades.Orm())
	extractor := appai.NewConfiguredOpenAITopicExtractor(repository)

	return newAssistantController(services.NewAssistantService(repository, extractor))
}

func newAssistantController(service assistantQuestionService) *AssistantController {
	return &AssistantController{service: service}
}

func (c *AssistantController) Ask(ctx http.Context) http.Response {
	userID, err := httpsupport.CurrentUserID(ctx)
	if err != nil {
		return demoUserRequiredResponse(ctx)
	}

	question, err := decodeAssistantQuestion(ctx)
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

	question = strings.TrimSpace(question)
	switch {
	case question == "":
		return assistantValidationResponse(ctx, "Câu hỏi là bắt buộc.")
	case utf8.RuneCountInString(question) > maximumAssistantQuestionLength:
		return assistantValidationResponse(ctx, "Câu hỏi không được vượt quá 500 ký tự.")
	}

	result, err := c.service.Ask(ctx, userID, question)
	if errors.Is(err, services.ErrDemoUserRequired) {
		return demoUserRequiredResponse(ctx)
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

func decodeAssistantQuestion(ctx http.Context) (string, error) {
	request := ctx.Request().Origin()
	if request == nil || request.Body == nil {
		return "", errMalformedAssistantJSON
	}
	if request.ContentLength > maximumAssistantRequestBytes {
		return "", errMalformedAssistantJSON
	}

	var payload struct {
		Question *string `json:"question"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, maximumAssistantRequestBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return "", err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return "", errMalformedAssistantJSON
	}
	if payload.Question == nil {
		return "", nil
	}

	return *payload.Question, nil
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
	return responses.Error(
		ctx,
		http.StatusUnprocessableEntity,
		"VALIDATION_ERROR",
		"Dữ liệu không hợp lệ",
		http.Json{
			"fields": http.Json{
				"question": []string{message},
			},
		},
	)
}
