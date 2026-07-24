package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	contractshttp "github.com/goravel/framework/contracts/http"

	"goravel/app/services"
)

type assistantQuestionServiceFake struct {
	response services.AssistantResponse
	err      error
	userID   uint64
	question string
	calls    int
}

func (f *assistantQuestionServiceFake) Ask(_ context.Context, userID uint64, question string) (services.AssistantResponse, error) {
	f.calls++
	f.userID = userID
	f.question = question
	return f.response, f.err
}

func TestAssistantControllerReturnsDirectSuccessPayload(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{
		response: services.AssistantResponse{
			Status:   services.AssistantStatusAnswered,
			Intent:   services.AssistantIntentCountPostsByTopic,
			Answer:   "Hiện có 8 bài viết về chủ đề “Phong cảnh”.",
			Provider: services.AssistantProviderLocal,
			Result: &services.AssistantCountResult{
				Count: 8,
				Topic: services.AssistantTopic{
					ID:      2,
					Slug:    "phong-canh",
					Name:    "Phong cảnh",
					Aliases: []string{"cảnh vật", "landscape"},
				},
			},
		},
	}
	controller := newAssistantController(service)
	ctx, request, responseBuilder, rendered := assistantControllerMocks(t)
	origin := httptest.NewRequest("POST", "/api/v1/assistant/questions", strings.NewReader(
		`{"question":"  Có bao nhiêu bài về phong cảnh?  "}`,
	))

	request.header = "12"
	request.origin = origin

	response := controller.Ask(ctx)

	if response != rendered {
		t.Fatal("Ask() did not return the rendered response")
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusOK, service.response)
	if service.calls != 1 || service.userID != 12 || service.question != "Có bao nhiêu bài về phong cảnh?" {
		t.Fatalf("service call = count %d, user %d, question %q", service.calls, service.userID, service.question)
	}
}

func TestAssistantControllerRequiresDemoIdentity(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{}
	controller := newAssistantController(service)
	ctx, _, responseBuilder, rendered := assistantControllerMocks(t)

	expected := contractshttp.Json{
		"error": contractshttp.Json{
			"code":    "DEMO_USER_REQUIRED",
			"message": "Vui lòng chọn một tài khoản mẫu",
			"details": contractshttp.Json{"header": "X-User-ID"},
		},
	}

	response := controller.Ask(ctx)

	if response != rendered || service.calls != 0 {
		t.Fatalf("Ask() response = %#v, service calls = %d", response, service.calls)
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusUnauthorized, expected)
}

func TestAssistantControllerReturnsBadRequestForMalformedJSON(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{}
	controller := newAssistantController(service)
	ctx, request, responseBuilder, rendered := assistantControllerMocks(t)
	origin := httptest.NewRequest("POST", "/api/v1/assistant/questions", strings.NewReader(`{"question":`))

	request.header = "1"
	request.origin = origin
	expected := contractshttp.Json{
		"error": contractshttp.Json{
			"code":    "BAD_REQUEST",
			"message": "Không thể đọc dữ liệu gửi lên",
			"details": contractshttp.Json{},
		},
	}

	response := controller.Ask(ctx)

	if response != rendered || service.calls != 0 {
		t.Fatalf("Ask() response = %#v, service calls = %d", response, service.calls)
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusBadRequest, expected)
}

func TestAssistantControllerRejectsOversizedRequestBodyBeforeService(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{}
	controller := newAssistantController(service)
	ctx, request, responseBuilder, rendered := assistantControllerMocks(t)
	origin := httptest.NewRequest("POST", "/api/v1/assistant/questions", strings.NewReader(
		`{"question":"`+strings.Repeat("x", 5000)+`"}`,
	))

	request.header = "1"
	request.origin = origin
	expected := contractshttp.Json{
		"error": contractshttp.Json{
			"code":    "BAD_REQUEST",
			"message": "Không thể đọc dữ liệu gửi lên",
			"details": contractshttp.Json{},
		},
	}

	response := controller.Ask(ctx)

	if response != rendered || service.calls != 0 {
		t.Fatalf("Ask() response = %#v, service calls = %d", response, service.calls)
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusBadRequest, expected)
}

func TestAssistantControllerValidatesWhitespaceOnlyQuestion(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{}
	controller := newAssistantController(service)
	ctx, request, responseBuilder, rendered := assistantControllerMocks(t)
	origin := httptest.NewRequest("POST", "/api/v1/assistant/questions", strings.NewReader(`{"question":"   "}`))

	request.header = "1"
	request.origin = origin
	expected := contractshttp.Json{
		"error": contractshttp.Json{
			"code":    "VALIDATION_ERROR",
			"message": "Dữ liệu không hợp lệ",
			"details": contractshttp.Json{
				"fields": contractshttp.Json{
					"question": []string{"Câu hỏi là bắt buộc."},
				},
			},
		},
	}

	response := controller.Ask(ctx)

	if response != rendered || service.calls != 0 {
		t.Fatalf("Ask() response = %#v, service calls = %d", response, service.calls)
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusUnprocessableEntity, expected)
}

func TestAssistantControllerValidatesQuestionLengthByUnicodeCharacters(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{}
	controller := newAssistantController(service)
	ctx, request, responseBuilder, rendered := assistantControllerMocks(t)
	origin := httptest.NewRequest("POST", "/api/v1/assistant/questions", strings.NewReader(
		`{"question":"`+strings.Repeat("ạ", 501)+`"}`,
	))

	request.header = "1"
	request.origin = origin
	expected := contractshttp.Json{
		"error": contractshttp.Json{
			"code":    "VALIDATION_ERROR",
			"message": "Dữ liệu không hợp lệ",
			"details": contractshttp.Json{
				"fields": contractshttp.Json{
					"question": []string{"Câu hỏi không được vượt quá 500 ký tự."},
				},
			},
		},
	}

	response := controller.Ask(ctx)

	if response != rendered || service.calls != 0 {
		t.Fatalf("Ask() response = %#v, service calls = %d", response, service.calls)
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusUnprocessableEntity, expected)
}

func TestAssistantControllerReturnsValidationErrorForNonStringQuestion(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{}
	controller := newAssistantController(service)
	ctx, request, responseBuilder, rendered := assistantControllerMocks(t)
	origin := httptest.NewRequest("POST", "/api/v1/assistant/questions", strings.NewReader(`{"question":42}`))

	request.header = "1"
	request.origin = origin
	expected := contractshttp.Json{
		"error": contractshttp.Json{
			"code":    "VALIDATION_ERROR",
			"message": "Dữ liệu không hợp lệ",
			"details": contractshttp.Json{
				"fields": contractshttp.Json{
					"question": []string{"Câu hỏi phải là chuỗi văn bản."},
				},
			},
		},
	}

	response := controller.Ask(ctx)

	if response != rendered || service.calls != 0 {
		t.Fatalf("Ask() response = %#v, service calls = %d", response, service.calls)
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusUnprocessableEntity, expected)
}

func TestAssistantControllerReturnsUnauthorizedForUnknownDemoUser(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{err: services.ErrDemoUserRequired}
	controller := newAssistantController(service)
	ctx, request, responseBuilder, rendered := assistantControllerMocks(t)
	origin := httptest.NewRequest("POST", "/api/v1/assistant/questions", strings.NewReader(`{"question":"Đếm bài về phong cảnh"}`))

	request.header = "99"
	request.origin = origin
	expected := contractshttp.Json{
		"error": contractshttp.Json{
			"code":    "DEMO_USER_REQUIRED",
			"message": "Vui lòng chọn một tài khoản mẫu",
			"details": contractshttp.Json{"header": "X-User-ID"},
		},
	}

	response := controller.Ask(ctx)

	if response != rendered || service.calls != 1 {
		t.Fatalf("Ask() response = %#v, service calls = %d", response, service.calls)
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusUnauthorized, expected)
}

func TestAssistantControllerMasksInternalFailure(t *testing.T) {
	t.Parallel()

	service := &assistantQuestionServiceFake{err: errors.New("database password must not leak")}
	controller := newAssistantController(service)
	ctx, request, responseBuilder, rendered := assistantControllerMocks(t)
	origin := httptest.NewRequest("POST", "/api/v1/assistant/questions", strings.NewReader(`{"question":"Đếm bài về phong cảnh"}`))

	request.header = "1"
	request.origin = origin
	expected := contractshttp.Json{
		"error": contractshttp.Json{
			"code":    "INTERNAL_ERROR",
			"message": "Đã xảy ra lỗi nội bộ, vui lòng thử lại sau",
			"details": contractshttp.Json{},
		},
	}

	response := controller.Ask(ctx)

	if response != rendered || service.calls != 1 {
		t.Fatalf("Ask() response = %#v, service calls = %d", response, service.calls)
	}
	assertAssistantHTTPResponse(t, responseBuilder, contractshttp.StatusInternalServerError, expected)
}

func assistantControllerMocks(t *testing.T) (
	*assistantHTTPContextFake,
	*assistantHTTPRequestFake,
	*assistantHTTPResponseFake,
	contractshttp.AbortableResponse,
) {
	t.Helper()

	rendered := &assistantRenderedResponseFake{}
	request := &assistantHTTPRequestFake{}
	responseBuilder := &assistantHTTPResponseFake{rendered: rendered}
	ctx := &assistantHTTPContextFake{
		request:  request,
		response: responseBuilder,
	}

	return ctx, request, responseBuilder, rendered
}

type assistantHTTPContextFake struct {
	assistantHTTPContextFallback
	request  contractshttp.ContextRequest
	response contractshttp.ContextResponse
}

type assistantHTTPContextFallback interface {
	contractshttp.Context
}

func (f *assistantHTTPContextFake) Request() contractshttp.ContextRequest {
	return f.request
}

func (f *assistantHTTPContextFake) Response() contractshttp.ContextResponse {
	return f.response
}

type assistantHTTPRequestFake struct {
	contractshttp.ContextRequest
	header string
	origin *http.Request
}

func (f *assistantHTTPRequestFake) Header(key string, _ ...string) string {
	if key == "X-User-ID" {
		return f.header
	}
	return ""
}

func (f *assistantHTTPRequestFake) Origin() *http.Request {
	return f.origin
}

type assistantHTTPResponseFake struct {
	contractshttp.ContextResponse
	status   int
	payload  any
	rendered contractshttp.AbortableResponse
	headers  http.Header
}

func (f *assistantHTTPResponseFake) Header(key, value string) contractshttp.ContextResponse {
	if f.headers == nil {
		f.headers = make(http.Header)
	}
	f.headers.Set(key, value)
	return f
}

func (f *assistantHTTPResponseFake) Origin() contractshttp.ResponseOrigin {
	if f.headers == nil {
		f.headers = make(http.Header)
	}
	return &assistantHTTPResponseOriginFake{headers: f.headers}
}

func (f *assistantHTTPResponseFake) Json(status int, payload any) contractshttp.AbortableResponse {
	f.status = status
	f.payload = payload
	return f.rendered
}

type assistantRenderedResponseFake struct {
	contractshttp.AbortableResponse
}

type assistantHTTPResponseOriginFake struct {
	contractshttp.ResponseOrigin
	headers http.Header
}

func (f *assistantHTTPResponseOriginFake) Header() http.Header {
	return f.headers
}

func assertAssistantHTTPResponse(
	t *testing.T,
	response *assistantHTTPResponseFake,
	status int,
	payload any,
) {
	t.Helper()

	if response.status != status || !reflect.DeepEqual(response.payload, payload) {
		t.Fatalf(
			"JSON response = status %d payload %#v, want status %d payload %#v",
			response.status,
			response.payload,
			status,
			payload,
		)
	}
}
