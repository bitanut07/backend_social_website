package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/http/responses"
	"goravel/app/http/support"
	"goravel/app/repositories"
	"goravel/app/services"
)

const (
	defaultCommentPageSize = 20
	maxCommentPageSize     = 100
	maxCommentRequestBytes = 32 * 1024
	maxCommentBodyLength   = 3000
)

type commentService interface {
	List(
		ctx context.Context,
		currentUserID string,
		postID string,
		page int,
		pageSize int,
	) ([]repositories.Comment, int64, error)
	Create(
		ctx context.Context,
		currentUserID string,
		postID string,
		body string,
	) (repositories.Comment, error)
	Delete(
		ctx context.Context,
		currentUserID string,
		postID string,
		commentID string,
	) error
}

type CommentController struct {
	service commentService
}

func NewCommentController() *CommentController {
	repository := repositories.NewCommentRepository()
	return NewCommentControllerWithService(services.NewCommentService(repository))
}

func NewCommentControllerWithService(service commentService) *CommentController {
	return &CommentController{service: service}
}

func (c *CommentController) List(ctx http.Context) http.Response {
	currentUserID, err := support.CurrentUserID(ctx)
	if err != nil {
		return contentDemoIdentityFailure().respond(ctx)
	}

	postID, inputErr := parseRequiredResourceID(ctx.Request().Route("id"), "id")
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	page, pageSize, failure := parseCommentPagination(ctx.Request().Queries())
	if failure != nil {
		return failure.respond(ctx)
	}

	comments, totalItems, err := c.service.List(
		ctx,
		currentUserID,
		postID,
		page,
		pageSize,
	)
	if err != nil {
		return commentServiceFailure(err).respond(ctx)
	}

	return responses.Paginated(
		ctx,
		comments,
		responses.Page(totalItems, page, pageSize),
	)
}

func (c *CommentController) Create(ctx http.Context) http.Response {
	currentUserID, err := support.CurrentUserID(ctx)
	if err != nil {
		return contentDemoIdentityFailure().respond(ctx)
	}

	postID, inputErr := parseRequiredResourceID(ctx.Request().Route("id"), "id")
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	var bodyReader io.Reader
	if request := ctx.Request().Origin(); request != nil {
		bodyReader = request.Body
	}
	body, inputErr := decodeAndValidateCreateComment(bodyReader)
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	comment, err := c.service.Create(ctx, currentUserID, postID, body)
	if err != nil {
		return commentServiceFailure(err).respond(ctx)
	}

	return responses.Data(ctx, http.StatusCreated, comment)
}

func (c *CommentController) Delete(ctx http.Context) http.Response {
	currentUserID, err := support.CurrentUserID(ctx)
	if err != nil {
		return contentDemoIdentityFailure().respond(ctx)
	}

	postID, inputErr := parseRequiredResourceID(ctx.Request().Route("id"), "id")
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}
	commentID, inputErr := parseRequiredResourceID(
		ctx.Request().Route("commentId"),
		"commentId",
	)
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	if err = c.service.Delete(
		ctx,
		currentUserID,
		postID,
		commentID,
	); err != nil {
		return commentServiceFailure(err).respond(ctx)
	}

	return ctx.Response().NoContent(http.StatusNoContent)
}

func parseCommentPagination(
	query map[string]string,
) (int, int, *contentRequestFailure) {
	return contentPagination(query, defaultCommentPageSize, maxCommentPageSize)
}

type createCommentPayload struct {
	Body *string `json:"body"`
}

func decodeAndValidateCreateComment(reader io.Reader) (string, *inputError) {
	if reader == nil {
		return "", malformedInputError("", io.EOF)
	}

	raw, err := io.ReadAll(io.LimitReader(reader, maxCommentRequestBytes+1))
	if err != nil {
		return "", malformedInputError("", err)
	}
	if len(raw) > maxCommentRequestBytes {
		return "", malformedInputError(
			"",
			errors.New("comment request body is too large"),
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var payload createCommentPayload
	if err = decoder.Decode(&payload); err != nil {
		return "", classifyProfileJSONError(err)
	}

	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("comment request contains trailing JSON")
		}
		return "", malformedInputError("", err)
	}
	if keyErr := validateExactCreateCommentKeys(raw); keyErr != nil {
		return "", keyErr
	}

	fields := make(map[string][]string)
	body := validateRequiredString(
		payload.Body,
		"body",
		maxCommentBodyLength,
		fields,
	)
	if strings.ContainsRune(body, '\x00') {
		fields["body"] = []string{"Nội dung bình luận không được chứa ký tự U+0000."}
	}
	if len(fields) > 0 {
		return "", &inputError{
			Kind:   inputErrorValidation,
			Fields: fields,
		}
	}

	return body, nil
}

func validateExactCreateCommentKeys(raw []byte) *inputError {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return malformedInputError("", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return validationInputError(
			"request",
			"Dữ liệu phải là một object JSON.",
		)
	}

	seenBody := false
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return malformedInputError("", err)
		}
		key, ok := token.(string)
		if !ok {
			return malformedInputError("", errors.New("comment key is not a string"))
		}
		if key != "body" {
			return validationInputError(
				key,
				"Trường dữ liệu không được hỗ trợ.",
			)
		}
		if seenBody {
			return validationInputError(
				"body",
				"Trường body không được lặp lại.",
			)
		}
		seenBody = true

		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return malformedInputError("", err)
		}
	}

	if _, err = decoder.Token(); err != nil {
		return malformedInputError("", err)
	}
	return nil
}

func commentServiceFailure(err error) *contentRequestFailure {
	if errors.Is(err, services.ErrDemoUserNotFound) {
		return contentDemoIdentityFailure()
	}
	if errors.Is(err, services.ErrNotFound) {
		return &contentRequestFailure{
			status:  http.StatusNotFound,
			code:    "NOT_FOUND",
			message: "Không tìm thấy tài nguyên",
			details: http.Json{},
		}
	}
	if errors.Is(err, services.ErrForbidden) {
		return &contentRequestFailure{
			status:  http.StatusForbidden,
			code:    "FORBIDDEN",
			message: "Bạn không có quyền bình luận bài viết này",
			details: http.Json{},
		}
	}

	return &contentRequestFailure{
		status:  http.StatusInternalServerError,
		code:    "INTERNAL_ERROR",
		message: "Hệ thống đang gặp sự cố, vui lòng thử lại sau",
		details: http.Json{},
	}
}
