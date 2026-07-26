package controllers

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/repositories"
	"goravel/app/services"
)

const (
	commentControllerUserID    = "00000000-0000-4000-8000-000000000001"
	commentControllerPostID    = "20000000-0000-4000-8000-000000000001"
	commentControllerCommentID = "50000000-0000-4000-8000-000000000001"
)

func TestParseCommentPaginationUsesContractDefaultsAndMaximum(t *testing.T) {
	t.Parallel()

	page, pageSize, failure := parseCommentPagination(map[string]string{})
	if failure != nil {
		t.Fatalf("default pagination failure = %#v", failure)
	}
	if page != 1 || pageSize != 20 {
		t.Fatalf("default pagination = %d/%d, want 1/20", page, pageSize)
	}

	_, _, failure = parseCommentPagination(map[string]string{"pageSize": "101"})
	if failure == nil ||
		failure.status != http.StatusUnprocessableEntity ||
		failure.code != "VALIDATION_ERROR" {
		t.Fatalf("pageSize=101 failure = %#v", failure)
	}
}

func TestDecodeAndValidateCreateCommentTrimsAndCountsUnicodeCharacters(t *testing.T) {
	t.Parallel()

	body, inputErr := decodeAndValidateCreateComment(
		strings.NewReader(`{"body":"  Bài vẽ rất đẹp!  "}`),
	)
	if inputErr != nil {
		t.Fatalf("valid comment error = %v", inputErr)
	}
	if body != "Bài vẽ rất đẹp!" {
		t.Fatalf("trimmed body = %q", body)
	}

	exactlyThreeThousand := strings.Repeat("ộ", 3000)
	body, inputErr = decodeAndValidateCreateComment(
		strings.NewReader(`{"body":"` + exactlyThreeThousand + `"}`),
	)
	if inputErr != nil || body != exactlyThreeThousand {
		t.Fatalf("3000 Unicode characters should be valid: body len=%d err=%v", len([]rune(body)), inputErr)
	}

	tooLong := strings.Repeat("ộ", 3001)
	_, inputErr = decodeAndValidateCreateComment(
		strings.NewReader(`{"body":"` + tooLong + `"}`),
	)
	if inputErr == nil || inputErr.Kind != inputErrorValidation {
		t.Fatalf("3001 Unicode characters error = %#v", inputErr)
	}
}

func TestDecodeAndValidateCreateCommentRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{"body":"Xin chào","userId":"00000000-0000-4000-8000-000000000001"}`,
		`{"body":"Xin chào","html":"<b>Xin chào</b>"}`,
		`{"body":"Xin chào"} {}`,
	} {
		payload := payload
		t.Run(payload, func(t *testing.T) {
			t.Parallel()

			if _, inputErr := decodeAndValidateCreateComment(
				strings.NewReader(payload),
			); inputErr == nil {
				t.Fatal("strict comment body must reject unsupported JSON shape")
			}
		})
	}
}

func TestDecodeAndValidateCreateCommentRequiresOneExactBodyKey(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{"Body":"Xin chào"}`,
		`{"BODY":"Xin chào"}`,
		`{"body":"Lần một","body":"Lần hai"}`,
		`{"body":"Lần một","Body":"Lần hai"}`,
	} {
		payload := payload
		t.Run(payload, func(t *testing.T) {
			t.Parallel()

			_, inputErr := decodeAndValidateCreateComment(
				strings.NewReader(payload),
			)
			if inputErr == nil || inputErr.Kind != inputErrorValidation {
				t.Fatalf("input error = %#v, want validation error", inputErr)
			}
			failure := inputFailure(inputErr)
			if failure.status != http.StatusUnprocessableEntity {
				t.Fatalf("failure status = %d, want 422", failure.status)
			}
		})
	}
}

func TestDecodeAndValidateCreateCommentRejectsNullCharacter(t *testing.T) {
	t.Parallel()

	_, inputErr := decodeAndValidateCreateComment(
		strings.NewReader(`{"body":"Xin\u0000chào"}`),
	)

	if inputErr == nil || inputErr.Kind != inputErrorValidation {
		t.Fatalf("input error = %#v, want validation error", inputErr)
	}
	failure := inputFailure(inputErr)
	if failure.status != http.StatusUnprocessableEntity {
		t.Fatalf("failure status = %d, want 422", failure.status)
	}
}

func TestDecodeAndValidateCreateCommentRejectsMissingBlankAndWrongTypeBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		kind    inputErrorKind
	}{
		{name: "missing", payload: `{}`, kind: inputErrorValidation},
		{name: "blank", payload: `{"body":" \n\t "}`, kind: inputErrorValidation},
		{name: "wrong type", payload: `{"body":42}`, kind: inputErrorValidation},
		{name: "malformed", payload: `{"body":`, kind: inputErrorMalformed},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, inputErr := decodeAndValidateCreateComment(
				strings.NewReader(tt.payload),
			)
			if inputErr == nil || inputErr.Kind != tt.kind {
				t.Fatalf("input error = %#v, want kind %s", inputErr, tt.kind)
			}
		})
	}
}

func TestCommentServiceFailureMapsIdentityPostAndPolicyErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "demo identity",
			err:        services.ErrDemoUserNotFound,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "DEMO_USER_REQUIRED",
		},
		{
			name:       "post missing",
			err:        services.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "comment policy",
			err:        services.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := commentServiceFailure(tt.err)
			if failure.status != tt.wantStatus || failure.code != tt.wantCode {
				t.Fatalf(
					"failure = %#v, want status/code %d/%s",
					failure,
					tt.wantStatus,
					tt.wantCode,
				)
			}
		})
	}
}

func TestCommentControllerDeleteReturnsNoContentForOwnedComment(t *testing.T) {
	t.Parallel()

	service := &commentControllerServiceFake{}
	controller := NewCommentControllerWithService(service)
	ctx, response, rendered := commentControllerDeleteContext(
		commentControllerPostID,
		commentControllerCommentID,
	)

	result := controller.Delete(ctx)

	if result != rendered {
		t.Fatalf("Delete response = %#v, want rendered response", result)
	}
	if response.status != http.StatusNoContent {
		t.Fatalf("Delete status = %d, want 204", response.status)
	}
	if !reflect.DeepEqual(service.deleteRequests, []commentControllerDeleteRequest{{
		userID:    commentControllerUserID,
		postID:    commentControllerPostID,
		commentID: commentControllerCommentID,
	}}) {
		t.Fatalf("Delete service requests = %#v", service.deleteRequests)
	}
}

func TestCommentControllerDeleteValidatesBothRouteUUIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		postID        string
		commentID     string
		wantParameter string
	}{
		{
			name:          "invalid post",
			postID:        "post",
			commentID:     commentControllerCommentID,
			wantParameter: "id",
		},
		{
			name:          "invalid comment",
			postID:        commentControllerPostID,
			commentID:     "comment",
			wantParameter: "commentId",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &commentControllerServiceFake{}
			controller := NewCommentControllerWithService(service)
			ctx, response, _ := commentControllerDeleteContext(
				tt.postID,
				tt.commentID,
			)

			controller.Delete(ctx)

			if response.status != http.StatusBadRequest {
				t.Fatalf("Delete status = %d, want 400", response.status)
			}
			if len(service.deleteRequests) != 0 {
				t.Fatalf("Delete service requests = %#v, want none", service.deleteRequests)
			}
			expected := http.Json{
				"error": http.Json{
					"code":    "BAD_REQUEST",
					"message": "Không thể đọc dữ liệu gửi lên",
					"details": http.Json{"parameter": tt.wantParameter},
				},
			}
			if !reflect.DeepEqual(response.payload, expected) {
				t.Fatalf("Delete payload = %#v, want %#v", response.payload, expected)
			}
		})
	}
}

type commentControllerDeleteRequest struct {
	userID    string
	postID    string
	commentID string
}

type commentControllerServiceFake struct {
	deleteErr      error
	deleteRequests []commentControllerDeleteRequest
}

func (f *commentControllerServiceFake) List(
	context.Context,
	string,
	string,
	int,
	int,
) ([]repositories.Comment, int64, error) {
	return []repositories.Comment{}, 0, nil
}

func (f *commentControllerServiceFake) Create(
	context.Context,
	string,
	string,
	string,
) (repositories.Comment, error) {
	return repositories.Comment{}, nil
}

func (f *commentControllerServiceFake) Delete(
	_ context.Context,
	userID string,
	postID string,
	commentID string,
) error {
	f.deleteRequests = append(f.deleteRequests, commentControllerDeleteRequest{
		userID:    userID,
		postID:    postID,
		commentID: commentID,
	})
	return f.deleteErr
}

type commentHTTPResponseFake struct {
	*assistantHTTPResponseFake
}

func (f *commentHTTPResponseFake) NoContent(
	code ...int,
) http.AbortableResponse {
	f.status = http.StatusNoContent
	if len(code) > 0 {
		f.status = code[0]
	}
	return f.rendered
}

func commentControllerDeleteContext(
	postID string,
	commentID string,
) (
	*assistantHTTPContextFake,
	*commentHTTPResponseFake,
	http.AbortableResponse,
) {
	rendered := &assistantRenderedResponseFake{}
	request := &assistantHTTPRequestFake{
		header: commentControllerUserID,
		routes: map[string]string{
			"id":        postID,
			"commentId": commentID,
		},
	}
	response := &commentHTTPResponseFake{
		assistantHTTPResponseFake: &assistantHTTPResponseFake{
			rendered: rendered,
		},
	}
	ctx := &assistantHTTPContextFake{
		request:  request,
		response: response,
	}

	return ctx, response, rendered
}
