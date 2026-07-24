package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/http/responses"
	"goravel/app/http/support"
	"goravel/app/repositories"
	"goravel/app/services"
)

const (
	defaultMessagePage     = 1
	defaultMessagePageSize = 50
	maxMessagePageSize     = 100
	maxMessagePageNumber   = 100_000
	maxMessageBodyLength   = 2000
	maxMessageRequestBytes = 32 * 1024
)

type messageService interface {
	List(
		ctx context.Context,
		currentUserID string,
		peerID string,
		page int,
		pageSize int,
	) (services.MessageListResult, error)
	Create(
		ctx context.Context,
		currentUserID string,
		recipientID string,
		body string,
	) (services.MessageDTO, error)
}

type MessageController struct {
	service messageService
}

func NewMessageController() *MessageController {
	repository := repositories.NewMessageRepository()
	return &MessageController{
		service: services.NewMessageService(repository),
	}
}

func (c *MessageController) List(ctx http.Context) http.Response {
	currentUserID, err := support.CurrentUserID(ctx)
	if err != nil {
		return messageDemoIdentityFailure().respond(ctx)
	}

	params, failure := parseMessageListParams(ctx.Request().Queries())
	if failure != nil {
		return failure.respond(ctx)
	}

	result, err := c.service.List(
		ctx,
		currentUserID,
		params.PeerID,
		params.Page,
		params.PageSize,
	)
	if err != nil {
		return messageServiceFailure(err, "peerId").respond(ctx)
	}

	return responses.Paginated(
		ctx,
		result.Messages,
		responses.Page(result.TotalItems, params.Page, params.PageSize),
	)
}

func (c *MessageController) Create(ctx http.Context) http.Response {
	currentUserID, err := support.CurrentUserID(ctx)
	if err != nil {
		return messageDemoIdentityFailure().respond(ctx)
	}

	var body io.Reader
	if request := ctx.Request().Origin(); request != nil {
		body = request.Body
	}
	input, err := decodeCreateMessageRequest(body)
	if err != nil {
		return messageJSONFailure(err).respond(ctx)
	}

	validated, failure := validateCreateMessageRequest(input)
	if failure != nil {
		return failure.respond(ctx)
	}

	message, err := c.service.Create(
		ctx,
		currentUserID,
		validated.RecipientID,
		validated.Body,
	)
	if err != nil {
		return messageServiceFailure(err, "recipientId").respond(ctx)
	}

	return responses.Data(ctx, http.StatusCreated, message)
}

type messageListParams struct {
	PeerID   string
	Page     int
	PageSize int
}

func parseMessageListParams(query map[string]string) (messageListParams, *messageRequestFailure) {
	peerValue, hasPeerID := query["peerId"]
	if !hasPeerID || strings.TrimSpace(peerValue) == "" {
		return messageListParams{}, messageValidationFailure(map[string][]string{
			"peerId": {"ID người trò chuyện là bắt buộc."},
		})
	}

	peerID, err := support.ParseResourceID(peerValue)
	if err != nil {
		return messageListParams{}, malformedMessageParameterFailure("peerId")
	}

	page, failure := parseOptionalMessagePageParameter(
		query,
		"page",
		defaultMessagePage,
		maxMessagePageNumber,
	)
	if failure != nil {
		return messageListParams{}, failure
	}
	pageSize, failure := parseOptionalMessagePageParameter(
		query,
		"pageSize",
		defaultMessagePageSize,
		maxMessagePageSize,
	)
	if failure != nil {
		return messageListParams{}, failure
	}

	return messageListParams{
		PeerID:   peerID,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func parseOptionalMessagePageParameter(
	query map[string]string,
	name string,
	defaultValue int,
	maximum int,
) (int, *messageRequestFailure) {
	value, exists := query[name]
	if !exists {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, malformedMessageParameterFailure(name)
	}
	if parsed < 1 {
		return 0, messageValidationFailure(map[string][]string{
			name: {"Giá trị phải là số nguyên dương."},
		})
	}
	if maximum > 0 && parsed > maximum {
		return 0, messageValidationFailure(map[string][]string{
			name: {fmt.Sprintf("Giá trị không được vượt quá %d.", maximum)},
		})
	}

	return parsed, nil
}

type createMessageRequest struct {
	RecipientID *string `json:"recipientId"`
	Body        *string `json:"body"`
}

type validatedCreateMessageRequest struct {
	RecipientID string
	Body        string
}

func decodeCreateMessageRequest(reader io.Reader) (createMessageRequest, error) {
	if reader == nil {
		return createMessageRequest{}, io.EOF
	}

	raw, err := io.ReadAll(io.LimitReader(reader, maxMessageRequestBytes+1))
	if err != nil {
		return createMessageRequest{}, err
	}
	if len(raw) > maxMessageRequestBytes {
		return createMessageRequest{}, errors.New("message request body is too large")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var input createMessageRequest
	if err := decoder.Decode(&input); err != nil {
		return createMessageRequest{}, classifyMessageJSONError(err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return createMessageRequest{}, errors.New("message request contains trailing JSON")
		}
		return createMessageRequest{}, err
	}

	return input, nil
}

func validateCreateMessageRequest(
	input createMessageRequest,
) (validatedCreateMessageRequest, *messageRequestFailure) {
	fields := make(map[string][]string)
	recipientID := ""
	if input.RecipientID == nil {
		fields["recipientId"] = []string{"ID người nhận là bắt buộc."}
	} else {
		parsedRecipientID, err := support.ParseResourceID(*input.RecipientID)
		if err != nil {
			fields["recipientId"] = []string{"ID người nhận phải là một UUID hợp lệ."}
		} else {
			recipientID = parsedRecipientID
		}
	}

	body := ""
	if input.Body == nil {
		fields["body"] = []string{"Nội dung tin nhắn là bắt buộc."}
	} else {
		body = strings.TrimSpace(*input.Body)
		switch {
		case body == "":
			fields["body"] = []string{"Nội dung tin nhắn không được để trống."}
		case utf8.RuneCountInString(body) > maxMessageBodyLength:
			fields["body"] = []string{"Nội dung tin nhắn không được vượt quá 2000 ký tự."}
		}
	}

	if len(fields) > 0 {
		return validatedCreateMessageRequest{}, messageValidationFailure(fields)
	}

	return validatedCreateMessageRequest{
		RecipientID: recipientID,
		Body:        body,
	}, nil
}

type messageJSONValidationError struct {
	field string
	err   error
}

func (e *messageJSONValidationError) Error() string {
	return e.err.Error()
}

func (e *messageJSONValidationError) Unwrap() error {
	return e.err
}

func classifyMessageJSONError(err error) error {
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		return &messageJSONValidationError{field: typeError.Field, err: err}
	}

	const unknownFieldPrefix = "json: unknown field "
	if strings.HasPrefix(err.Error(), unknownFieldPrefix) {
		field := strings.Trim(strings.TrimPrefix(err.Error(), unknownFieldPrefix), `"`)
		return &messageJSONValidationError{field: field, err: err}
	}

	return err
}

func messageJSONFailure(err error) *messageRequestFailure {
	var validationErr *messageJSONValidationError
	if errors.As(err, &validationErr) {
		field := validationErr.field
		if field == "" {
			field = "request"
		}
		return messageValidationFailure(map[string][]string{
			field: {"Giá trị không đúng định dạng."},
		})
	}

	return &messageRequestFailure{
		status:  http.StatusBadRequest,
		code:    "BAD_REQUEST",
		message: "Không thể đọc dữ liệu gửi lên",
		details: http.Json{},
	}
}

type messageRequestFailure struct {
	status  int
	code    string
	message string
	details any
}

func (f *messageRequestFailure) respond(ctx http.Context) http.Response {
	return responses.Error(ctx, f.status, f.code, f.message, f.details)
}

func malformedMessageParameterFailure(parameter string) *messageRequestFailure {
	return &messageRequestFailure{
		status:  http.StatusBadRequest,
		code:    "BAD_REQUEST",
		message: "Không thể đọc dữ liệu gửi lên",
		details: http.Json{"parameter": parameter},
	}
}

func messageValidationFailure(fields map[string][]string) *messageRequestFailure {
	return &messageRequestFailure{
		status:  http.StatusUnprocessableEntity,
		code:    "VALIDATION_ERROR",
		message: "Dữ liệu không hợp lệ",
		details: http.Json{"fields": fields},
	}
}

func messageDemoIdentityFailure() *messageRequestFailure {
	return &messageRequestFailure{
		status:  http.StatusUnauthorized,
		code:    "DEMO_USER_REQUIRED",
		message: "Vui lòng chọn một tài khoản mẫu",
		details: http.Json{"header": "X-User-ID"},
	}
}

func messageServiceFailure(err error, participantField string) *messageRequestFailure {
	switch {
	case errors.Is(err, services.ErrDemoUserNotFound):
		return messageDemoIdentityFailure()
	case errors.Is(err, services.ErrMessagePeerNotFound):
		return &messageRequestFailure{
			status:  http.StatusNotFound,
			code:    "NOT_FOUND",
			message: "Không tìm thấy tài nguyên",
			details: http.Json{},
		}
	case errors.Is(err, services.ErrCannotMessageSelf):
		return messageValidationFailure(map[string][]string{
			participantField: {"Người trò chuyện phải khác tài khoản hiện tại."},
		})
	default:
		return &messageRequestFailure{
			status:  http.StatusInternalServerError,
			code:    "INTERNAL_ERROR",
			message: "Hệ thống đang gặp sự cố, vui lòng thử lại sau",
			details: http.Json{},
		}
	}
}
