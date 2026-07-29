package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
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
	defaultDirectoryPageSize = 20
	maxDirectoryPageSize     = 100
	defaultPostPageSize      = 10
	maxPostPageSize          = 50
	maximumPageNumber        = 100_000
	maxPostRequestBytes      = 64 * 1024
	maxProfileRequestBytes   = 16 * 1024
	maxDemoLoginRequestBytes = 8 * 1024
)

var profileUsernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,50}$`)

type ContentServiceContract interface {
	ListUsers(ctx context.Context, page, pageSize int) ([]repositories.User, int64, error)
	DemoLogin(ctx context.Context, input services.DemoLoginInput) (repositories.User, error)
	UpdateProfile(
		ctx context.Context,
		userID string,
		input repositories.UpdateProfileInput,
	) (repositories.User, error)
	ListTopics(ctx context.Context, page, pageSize int) ([]repositories.Topic, int64, error)
	ListPosts(
		ctx context.Context,
		viewerID string,
		page, pageSize int,
		topicID, authorID *string,
	) ([]repositories.Post, int64, error)
	CreatePost(
		ctx context.Context,
		userID string,
		input repositories.CreatePostInput,
	) (repositories.Post, error)
	DeletePost(ctx context.Context, userID, postID string) error
	PutReaction(ctx context.Context, userID, postID string) (repositories.ReactionState, error)
	DeleteReaction(ctx context.Context, userID, postID string) (repositories.ReactionState, error)
}

type ContentController struct {
	service ContentServiceContract
}

func NewContentController() *ContentController {
	repository := repositories.NewContentRepository()
	return NewContentControllerWithService(services.NewContentService(repository))
}

func NewContentControllerWithService(service ContentServiceContract) *ContentController {
	return &ContentController{service: service}
}

func (c *ContentController) ListUsers(ctx http.Context) http.Response {
	page, pageSize, failure := contentPagination(
		ctx.Request().Queries(),
		defaultDirectoryPageSize,
		maxDirectoryPageSize,
	)
	if failure != nil {
		return failure.respond(ctx)
	}

	users, totalItems, err := c.service.ListUsers(ctx, page, pageSize)
	if err != nil {
		return contentServiceFailure(err).respond(ctx)
	}

	return responses.Paginated(ctx, users, responses.Page(totalItems, page, pageSize))
}

func (c *ContentController) CreateDemoSession(ctx http.Context) http.Response {
	var body io.Reader
	if request := ctx.Request().Origin(); request != nil {
		body = request.Body
	}
	input, inputErr := decodeAndValidateDemoLogin(body)
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	user, err := c.service.DemoLogin(ctx, input)
	if err != nil {
		return contentServiceFailure(err).respond(ctx)
	}

	return responses.Data(ctx, http.StatusCreated, user)
}

func (c *ContentController) UpdateProfile(ctx http.Context) http.Response {
	userID, err := support.CurrentUserID(ctx)
	if err != nil {
		return contentDemoIdentityFailure().respond(ctx)
	}

	var body io.Reader
	if request := ctx.Request().Origin(); request != nil {
		body = request.Body
	}
	input, inputErr := decodeAndValidateUpdateProfile(body)
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	user, err := c.service.UpdateProfile(ctx, userID, input)
	if err != nil {
		return contentServiceFailure(err).respond(ctx)
	}

	return responses.Data(ctx, http.StatusOK, user)
}

func (c *ContentController) ListTopics(ctx http.Context) http.Response {
	page, pageSize, failure := contentPagination(
		ctx.Request().Queries(),
		defaultDirectoryPageSize,
		maxDirectoryPageSize,
	)
	if failure != nil {
		return failure.respond(ctx)
	}

	topics, totalItems, err := c.service.ListTopics(ctx, page, pageSize)
	if err != nil {
		return contentServiceFailure(err).respond(ctx)
	}

	return responses.Paginated(ctx, topics, responses.Page(totalItems, page, pageSize))
}

func (c *ContentController) ListPosts(ctx http.Context) http.Response {
	viewerID, err := support.CurrentUserID(ctx)
	if err != nil {
		return contentDemoIdentityFailure().respond(ctx)
	}

	query := ctx.Request().Queries()
	page, pageSize, failure := contentPagination(query, defaultPostPageSize, maxPostPageSize)
	if failure != nil {
		return failure.respond(ctx)
	}

	topicRaw := ""
	if value, exists := query["topicId"]; exists {
		if strings.TrimSpace(value) == "" {
			return malformedContentParameterFailure("topicId").respond(ctx)
		}
		topicRaw = value
	}
	topicID, inputErr := parseOptionalResourceID(topicRaw, "topicId")
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	authorRaw := ""
	if value, exists := query["authorId"]; exists {
		if strings.TrimSpace(value) == "" {
			return malformedContentParameterFailure("authorId").respond(ctx)
		}
		authorRaw = value
	}
	authorID, inputErr := parseOptionalResourceID(authorRaw, "authorId")
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	posts, totalItems, err := c.service.ListPosts(ctx, viewerID, page, pageSize, topicID, authorID)
	if err != nil {
		return contentServiceFailure(err).respond(ctx)
	}

	return responses.Paginated(ctx, posts, responses.Page(totalItems, page, pageSize))
}

func (c *ContentController) CreatePost(ctx http.Context) http.Response {
	userID, err := support.CurrentUserID(ctx)
	if err != nil {
		return contentDemoIdentityFailure().respond(ctx)
	}

	var body io.Reader
	if request := ctx.Request().Origin(); request != nil {
		body = request.Body
	}
	input, inputErr := decodeAndValidateCreatePost(body)
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	post, err := c.service.CreatePost(ctx, userID, input)
	if err != nil {
		return contentServiceFailure(err).respond(ctx)
	}

	return responses.Data(ctx, http.StatusCreated, post)
}

func (c *ContentController) DeletePost(ctx http.Context) http.Response {
	userID, err := support.CurrentUserID(ctx)
	if err != nil {
		return contentDemoIdentityFailure().respond(ctx)
	}

	postID, inputErr := parseRequiredResourceID(
		ctx.Request().Route("id"),
		"id",
	)
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	if err = c.service.DeletePost(ctx, userID, postID); err != nil {
		return contentServiceFailure(err).respond(ctx)
	}

	return ctx.Response().NoContent(http.StatusNoContent)
}

func (c *ContentController) PutReaction(ctx http.Context) http.Response {
	return c.changeReaction(ctx, true)
}

func (c *ContentController) DeleteReaction(ctx http.Context) http.Response {
	return c.changeReaction(ctx, false)
}

func (c *ContentController) changeReaction(ctx http.Context, put bool) http.Response {
	userID, err := support.CurrentUserID(ctx)
	if err != nil {
		return contentDemoIdentityFailure().respond(ctx)
	}

	postID, inputErr := parseRequiredResourceID(ctx.Request().Route("id"), "id")
	if inputErr != nil {
		return inputFailure(inputErr).respond(ctx)
	}

	var state repositories.ReactionState
	if put {
		state, err = c.service.PutReaction(ctx, userID, postID)
	} else {
		state, err = c.service.DeleteReaction(ctx, userID, postID)
	}
	if err != nil {
		return contentServiceFailure(err).respond(ctx)
	}

	return responses.Data(ctx, http.StatusOK, state)
}

type inputErrorKind string

const (
	inputErrorMalformed  inputErrorKind = "MALFORMED"
	inputErrorValidation inputErrorKind = "VALIDATION"
)

type inputError struct {
	Kind   inputErrorKind
	Field  string
	Fields map[string][]string
	err    error
}

func (e *inputError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return string(e.Kind)
}

func parsePagination(
	pageRaw, pageSizeRaw string,
	defaultPageSize, maximumPageSize int,
) (int, int, *inputError) {
	page, err := parsePositiveInt(pageRaw, 1, maximumPageNumber, "page")
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := parsePositiveInt(
		pageSizeRaw,
		defaultPageSize,
		maximumPageSize,
		"pageSize",
	)
	if err != nil {
		return 0, 0, err
	}

	maxInt := int(^uint(0) >> 1)
	if page > maxInt/pageSize {
		return 0, 0, validationInputError("page", "Giá trị trang quá lớn.")
	}

	return page, pageSize, nil
}

func parsePositiveInt(raw string, defaultValue, maximum int, field string) (int, *inputError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, malformedInputError(field, err)
	}
	if parsed < 1 {
		return 0, validationInputError(field, "Giá trị phải là số nguyên dương.")
	}
	if maximum > 0 && parsed > maximum {
		return 0, validationInputError(
			field,
			fmt.Sprintf("Giá trị không được vượt quá %d.", maximum),
		)
	}

	return parsed, nil
}

func contentPagination(
	query map[string]string,
	defaultPageSize, maximumPageSize int,
) (int, int, *contentRequestFailure) {
	pageRaw, pageSizeRaw := "", ""
	if value, exists := query["page"]; exists {
		if strings.TrimSpace(value) == "" {
			return 0, 0, malformedContentParameterFailure("page")
		}
		pageRaw = value
	}
	if value, exists := query["pageSize"]; exists {
		if strings.TrimSpace(value) == "" {
			return 0, 0, malformedContentParameterFailure("pageSize")
		}
		pageSizeRaw = value
	}

	page, pageSize, err := parsePagination(
		pageRaw,
		pageSizeRaw,
		defaultPageSize,
		maximumPageSize,
	)
	if err != nil {
		return 0, 0, inputFailure(err)
	}

	return page, pageSize, nil
}

func parseOptionalResourceID(raw, field string) (*string, *inputError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	id, err := support.ParseResourceID(value)
	if err != nil {
		return nil, malformedInputError(field, err)
	}

	return &id, nil
}

func parseRequiredResourceID(raw, field string) (string, *inputError) {
	if strings.TrimSpace(raw) == "" {
		return "", malformedInputError(field, errors.New("missing resource id"))
	}

	id, err := parseOptionalResourceID(raw, field)
	if err != nil {
		return "", err
	}
	if id == nil {
		return "", malformedInputError(field, errors.New("missing resource id"))
	}

	return *id, nil
}

type createPostPayload struct {
	Title    *string   `json:"title"`
	Caption  *string   `json:"caption"`
	ImageURL *string   `json:"imageUrl"`
	ExamName *string   `json:"examName"`
	TopicIDs *[]string `json:"topicIds"`
}

type updateProfilePayload struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"displayName"`
	AvatarURL   *string `json:"avatarUrl"`
}

type demoLoginPayload struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
}

func decodeAndValidateDemoLogin(reader io.Reader) (services.DemoLoginInput, *inputError) {
	if reader == nil {
		return services.DemoLoginInput{}, malformedInputError("", io.EOF)
	}

	raw, err := io.ReadAll(io.LimitReader(reader, maxDemoLoginRequestBytes+1))
	if err != nil {
		return services.DemoLoginInput{}, malformedInputError("", err)
	}
	if len(raw) > maxDemoLoginRequestBytes {
		return services.DemoLoginInput{}, malformedInputError(
			"",
			errors.New("request body is too large"),
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var payload demoLoginPayload
	if err = decoder.Decode(&payload); err != nil {
		return services.DemoLoginInput{}, classifyProfileJSONError(err)
	}

	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("request contains trailing JSON")
		}
		return services.DemoLoginInput{}, malformedInputError("", err)
	}

	return validateDemoLoginPayload(payload)
}

func validateDemoLoginPayload(payload demoLoginPayload) (services.DemoLoginInput, *inputError) {
	fields := make(map[string][]string)

	rawUsername := validateRequiredString(payload.Username, "username", 50, fields)
	username := strings.ToLower(strings.TrimPrefix(rawUsername, "@"))
	switch {
	case username == "" && rawUsername != "":
		fields["username"] = []string{"Username không hợp lệ."}
	case username != "" && !profileUsernamePattern.MatchString(username):
		fields["username"] = []string{
			"Username chỉ gồm chữ thường, số, dấu chấm, gạch dưới hoặc gạch ngang.",
		}
	}
	password := validateRequiredString(payload.Password, "password", 128, fields)

	if len(fields) > 0 {
		return services.DemoLoginInput{}, &inputError{
			Kind:   inputErrorValidation,
			Fields: fields,
		}
	}

	return services.DemoLoginInput{
		Username: username,
		Password: password,
	}, nil
}

func decodeAndValidateUpdateProfile(reader io.Reader) (repositories.UpdateProfileInput, *inputError) {
	if reader == nil {
		return repositories.UpdateProfileInput{}, malformedInputError("", io.EOF)
	}

	raw, err := io.ReadAll(io.LimitReader(reader, maxProfileRequestBytes+1))
	if err != nil {
		return repositories.UpdateProfileInput{}, malformedInputError("", err)
	}
	if len(raw) > maxProfileRequestBytes {
		return repositories.UpdateProfileInput{}, malformedInputError(
			"",
			errors.New("request body is too large"),
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var payload updateProfilePayload
	if err = decoder.Decode(&payload); err != nil {
		return repositories.UpdateProfileInput{}, classifyProfileJSONError(err)
	}

	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("request contains trailing JSON")
		}
		return repositories.UpdateProfileInput{}, malformedInputError("", err)
	}

	return validateUpdateProfilePayload(payload)
}

func validateUpdateProfilePayload(
	payload updateProfilePayload,
) (repositories.UpdateProfileInput, *inputError) {
	fields := make(map[string][]string)

	username := validateRequiredString(payload.Username, "username", 50, fields)
	switch {
	case username != "" && utf8.RuneCountInString(username) < 3:
		fields["username"] = []string{"Username phải có ít nhất 3 ký tự."}
	case username != "" && !profileUsernamePattern.MatchString(username):
		fields["username"] = []string{
			"Username chỉ gồm chữ thường, số, dấu chấm, gạch dưới hoặc gạch ngang.",
		}
	}

	displayName := validateRequiredString(payload.DisplayName, "displayName", 100, fields)

	var avatarURL *string
	if payload.AvatarURL != nil {
		trimmed := strings.TrimSpace(*payload.AvatarURL)
		switch {
		case trimmed == "":
			avatarURL = nil
		case utf8.RuneCountInString(trimmed) > 2048:
			fields["avatarUrl"] = []string{"URL avatar không được vượt quá 2048 ký tự."}
		case !validHTTPURL(trimmed):
			fields["avatarUrl"] = []string{"URL avatar phải là URL HTTP hoặc HTTPS tuyệt đối."}
		default:
			avatarURL = &trimmed
		}
	}

	if len(fields) > 0 {
		return repositories.UpdateProfileInput{}, &inputError{
			Kind:   inputErrorValidation,
			Fields: fields,
		}
	}

	return repositories.UpdateProfileInput{
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}, nil
}

func decodeAndValidateCreatePost(reader io.Reader) (repositories.CreatePostInput, *inputError) {
	if reader == nil {
		return repositories.CreatePostInput{}, malformedInputError("", io.EOF)
	}

	raw, err := io.ReadAll(io.LimitReader(reader, maxPostRequestBytes+1))
	if err != nil {
		return repositories.CreatePostInput{}, malformedInputError("", err)
	}
	if len(raw) > maxPostRequestBytes {
		return repositories.CreatePostInput{}, malformedInputError(
			"",
			errors.New("request body is too large"),
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var payload createPostPayload
	if err = decoder.Decode(&payload); err != nil {
		return repositories.CreatePostInput{}, classifyCreatePostJSONError(err)
	}

	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("request contains trailing JSON")
		}
		return repositories.CreatePostInput{}, malformedInputError("", err)
	}

	rawFields := make(map[string]json.RawMessage)
	if err = json.Unmarshal(raw, &rawFields); err != nil {
		return repositories.CreatePostInput{}, malformedInputError("", err)
	}
	if examName, exists := rawFields["examName"]; exists &&
		bytes.Equal(bytes.TrimSpace(examName), []byte("null")) {
		return repositories.CreatePostInput{}, validationInputError(
			"examName",
			"Tên cuộc thi không được là null.",
		)
	}

	return validateCreatePostPayload(payload)
}

func validateCreatePostPayload(
	payload createPostPayload,
) (repositories.CreatePostInput, *inputError) {
	fields := make(map[string][]string)

	title := validateRequiredString(payload.Title, "title", 120, fields)
	caption := validateRequiredString(payload.Caption, "caption", 2000, fields)
	imageURL := validateRequiredString(payload.ImageURL, "imageUrl", 2048, fields)
	if imageURL != "" && !validHTTPURL(imageURL) {
		fields["imageUrl"] = []string{"URL ảnh phải là URL HTTP hoặc HTTPS tuyệt đối."}
	}

	var examName *string
	if payload.ExamName != nil {
		trimmed := strings.TrimSpace(*payload.ExamName)
		switch {
		case trimmed == "":
			fields["examName"] = []string{"Tên cuộc thi không được để trống."}
		case utf8.RuneCountInString(trimmed) > 160:
			fields["examName"] = []string{"Tên cuộc thi không được vượt quá 160 ký tự."}
		default:
			examName = &trimmed
		}
	}

	topicIDs := make([]string, 0)
	if payload.TopicIDs == nil {
		fields["topicIds"] = []string{"Danh sách chủ đề là bắt buộc."}
	} else {
		switch {
		case len(*payload.TopicIDs) < 1:
			fields["topicIds"] = []string{"Cần chọn ít nhất một chủ đề."}
		case len(*payload.TopicIDs) > 5:
			fields["topicIds"] = []string{"Chỉ được chọn tối đa năm chủ đề."}
		default:
			seen := make(map[string]struct{}, len(*payload.TopicIDs))
			for _, rawTopicID := range *payload.TopicIDs {
				topicID, err := support.ParseResourceID(rawTopicID)
				if err != nil {
					fields["topicIds"] = []string{"Mỗi ID chủ đề phải là một UUID hợp lệ."}
					break
				}
				if _, exists := seen[topicID]; exists {
					fields["topicIds"] = []string{"Các ID chủ đề không được trùng nhau."}
					break
				}
				seen[topicID] = struct{}{}
				topicIDs = append(topicIDs, topicID)
			}
		}
	}

	if len(fields) > 0 {
		return repositories.CreatePostInput{}, &inputError{
			Kind:   inputErrorValidation,
			Fields: fields,
		}
	}

	return repositories.CreatePostInput{
		Title:    title,
		Caption:  caption,
		ImageURL: imageURL,
		ExamName: examName,
		TopicIDs: topicIDs,
	}, nil
}

func validateRequiredString(
	value *string,
	field string,
	maximum int,
	fields map[string][]string,
) string {
	if value == nil {
		fields[field] = []string{"Trường này là bắt buộc."}
		return ""
	}

	trimmed := strings.TrimSpace(*value)
	switch {
	case trimmed == "":
		fields[field] = []string{"Giá trị không được để trống."}
	case utf8.RuneCountInString(trimmed) > maximum:
		fields[field] = []string{fmt.Sprintf("Giá trị không được vượt quá %d ký tự.", maximum)}
	}

	return trimmed
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}

	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func classifyCreatePostJSONError(err error) *inputError {
	return classifyProfileJSONError(err)
}

func classifyProfileJSONError(err error) *inputError {
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		field := typeError.Field
		if field == "" {
			field = "request"
		}
		return validationInputError(field, "Giá trị không đúng định dạng.")
	}

	const unknownFieldPrefix = "json: unknown field "
	if strings.HasPrefix(err.Error(), unknownFieldPrefix) {
		field := strings.Trim(strings.TrimPrefix(err.Error(), unknownFieldPrefix), `"`)
		return validationInputError(field, "Trường dữ liệu không được hỗ trợ.")
	}

	return malformedInputError("", err)
}

func malformedInputError(field string, err error) *inputError {
	return &inputError{
		Kind:  inputErrorMalformed,
		Field: field,
		err:   err,
	}
}

func validationInputError(field, message string) *inputError {
	return &inputError{
		Kind: inputErrorValidation,
		Fields: map[string][]string{
			field: {message},
		},
	}
}

func inputFailure(err *inputError) *contentRequestFailure {
	if err.Kind == inputErrorValidation {
		return contentValidationFailure(err.Fields)
	}
	if err.Field != "" {
		return malformedContentParameterFailure(err.Field)
	}

	return &contentRequestFailure{
		status:  http.StatusBadRequest,
		code:    "BAD_REQUEST",
		message: "Không thể đọc dữ liệu gửi lên",
		details: http.Json{},
	}
}

func contentServiceFailure(err error) *contentRequestFailure {
	if errors.Is(err, services.ErrDemoUserNotFound) {
		return contentDemoIdentityFailure()
	}
	if errors.Is(err, services.ErrInvalidDemoCredentials) {
		return &contentRequestFailure{
			status:  http.StatusUnauthorized,
			code:    "INVALID_DEMO_CREDENTIALS",
			message: "Sai tài khoản hoặc mật khẩu demo",
			details: http.Json{},
		}
	}

	var missingTopics *services.MissingTopicsError
	if errors.As(err, &missingTopics) {
		return &contentRequestFailure{
			status:  http.StatusNotFound,
			code:    "NOT_FOUND",
			message: "Không tìm thấy tài nguyên",
			details: http.Json{"topicIds": missingTopics.TopicIDs},
		}
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
			message: "Bạn không có quyền xóa bài viết này",
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

type contentRequestFailure struct {
	status  int
	code    string
	message string
	details any
}

func (f *contentRequestFailure) respond(ctx http.Context) http.Response {
	return responses.Error(ctx, f.status, f.code, f.message, f.details)
}

func malformedContentParameterFailure(parameter string) *contentRequestFailure {
	return &contentRequestFailure{
		status:  http.StatusBadRequest,
		code:    "BAD_REQUEST",
		message: "Không thể đọc dữ liệu gửi lên",
		details: http.Json{"parameter": parameter},
	}
}

func contentValidationFailure(fields map[string][]string) *contentRequestFailure {
	return &contentRequestFailure{
		status:  http.StatusUnprocessableEntity,
		code:    "VALIDATION_ERROR",
		message: "Dữ liệu không hợp lệ",
		details: http.Json{"fields": fields},
	}
}

func contentDemoIdentityFailure() *contentRequestFailure {
	return &contentRequestFailure{
		status:  http.StatusUnauthorized,
		code:    "DEMO_USER_REQUIRED",
		message: "Vui lòng chọn một tài khoản mẫu",
		details: http.Json{"header": "X-User-ID"},
	}
}
