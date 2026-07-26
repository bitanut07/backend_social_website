package controllers

import (
	"strings"
	"testing"

	"goravel/app/services"
)

const (
	contentControllerTopicID1      = "10000000-0000-4000-8000-000000000001"
	contentControllerTopicID2      = "10000000-0000-4000-8000-000000000002"
	contentControllerCanonicalUUID = "10000000-0000-4000-8000-00000000000a"
	contentControllerUppercaseUUID = "10000000-0000-4000-8000-00000000000A"
	contentControllerNilUUID       = "00000000-0000-0000-0000-000000000000"
)

func TestParsePagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		page         string
		pageSize     string
		defaultSize  int
		maxSize      int
		wantPage     int
		wantPageSize int
		wantKind     inputErrorKind
	}{
		{
			name:         "uses documented defaults",
			defaultSize:  10,
			maxSize:      50,
			wantPage:     1,
			wantPageSize: 10,
		},
		{
			name:         "accepts valid values",
			page:         "2",
			pageSize:     "25",
			defaultSize:  10,
			maxSize:      50,
			wantPage:     2,
			wantPageSize: 25,
		},
		{
			name:        "rejects malformed page as bad request",
			page:        "two",
			defaultSize: 10,
			maxSize:     50,
			wantKind:    inputErrorMalformed,
		},
		{
			name:        "rejects zero page as validation error",
			page:        "0",
			defaultSize: 10,
			maxSize:     50,
			wantKind:    inputErrorValidation,
		},
		{
			name:        "rejects page size above endpoint maximum",
			pageSize:    "51",
			defaultSize: 10,
			maxSize:     50,
			wantKind:    inputErrorValidation,
		},
		{
			name:        "rejects page above global maximum",
			page:        "100001",
			defaultSize: 10,
			maxSize:     50,
			wantKind:    inputErrorValidation,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			page, pageSize, err := parsePagination(tt.page, tt.pageSize, tt.defaultSize, tt.maxSize)
			if tt.wantKind != "" {
				if err == nil {
					t.Fatalf("expected %s error, got nil", tt.wantKind)
				}
				if err.Kind != tt.wantKind {
					t.Fatalf("got error kind %q, want %q", err.Kind, tt.wantKind)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if page != tt.wantPage || pageSize != tt.wantPageSize {
				t.Fatalf("got page=%d pageSize=%d, want page=%d pageSize=%d",
					page, pageSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}

func TestDecodeAndValidateCreatePost(t *testing.T) {
	t.Parallel()

	validBody := `{
		"title": "  Bình minh  ",
		"caption": "  Màu nước trên giấy.  ",
		"imageUrl": " https://images.example.com/art.jpg ",
		"examName": "  Sắc màu 2026  ",
		"topicIds": [
			"10000000-0000-4000-8000-000000000001",
			"10000000-0000-4000-8000-000000000002"
		]
	}`

	request, err := decodeAndValidateCreatePost(strings.NewReader(validBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if request.Title != "Bình minh" {
		t.Fatalf("got title %q, want trimmed title", request.Title)
	}
	if request.Caption != "Màu nước trên giấy." {
		t.Fatalf("got caption %q, want trimmed caption", request.Caption)
	}
	if request.ImageURL != "https://images.example.com/art.jpg" {
		t.Fatalf("got image URL %q, want trimmed URL", request.ImageURL)
	}
	if request.ExamName == nil || *request.ExamName != "Sắc màu 2026" {
		t.Fatalf("unexpected exam name: %#v", request.ExamName)
	}
	if len(request.TopicIDs) != 2 ||
		request.TopicIDs[0] != contentControllerTopicID1 ||
		request.TopicIDs[1] != contentControllerTopicID2 {
		t.Fatalf("unexpected topic ids: %#v", request.TopicIDs)
	}
}

func TestDecodeAndValidateCreatePostCanonicalizesTopicUUIDs(t *testing.T) {
	t.Parallel()

	body := `{
		"title":"T",
		"caption":"C",
		"imageUrl":"https://example.com/a.jpg",
		"topicIds":["10000000-0000-4000-8000-00000000000A"]
	}`

	request, err := decodeAndValidateCreatePost(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !equalStringSlices(request.TopicIDs, []string{contentControllerCanonicalUUID}) {
		t.Fatalf(
			"topic IDs = %#v, want canonical UUID %q",
			request.TopicIDs,
			contentControllerCanonicalUUID,
		)
	}
}

func TestDecodeAndValidateDemoLogin(t *testing.T) {
	t.Parallel()

	request, err := decodeAndValidateDemoLogin(strings.NewReader(`{
		"username": " @THU.HA.CAFE ",
		"password": " artly-demo "
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if request.Username != "thu.ha.cafe" {
		t.Fatalf("username = %q, want normalized username", request.Username)
	}
	if request.Password != "artly-demo" {
		t.Fatalf("password = %q, want trimmed password", request.Password)
	}

	tests := []struct {
		name     string
		body     string
		wantKind inputErrorKind
	}{
		{
			name:     "rejects malformed JSON",
			body:     `{"username":`,
			wantKind: inputErrorMalformed,
		},
		{
			name: "rejects unknown fields",
			body: `{
				"username":"thu.ha.cafe",
				"password":"artly-demo",
				"role":"STUDENT"
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name:     "rejects blank username",
			body:     `{"username":" ","password":"artly-demo"}`,
			wantKind: inputErrorValidation,
		},
		{
			name:     "rejects blank password",
			body:     `{"username":"thu.ha.cafe","password":" "}`,
			wantKind: inputErrorValidation,
		},
		{
			name:     "rejects invalid username",
			body:     `{"username":"Thu Ha","password":"artly-demo"}`,
			wantKind: inputErrorValidation,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, inputErr := decodeAndValidateDemoLogin(strings.NewReader(tt.body))
			if inputErr == nil {
				t.Fatal("expected an error, got nil")
			}
			if inputErr.Kind != tt.wantKind {
				t.Fatalf("got error kind %q, want %q", inputErr.Kind, tt.wantKind)
			}
		})
	}
}

func TestDecodeAndValidateCreatePostRejectsMalformedShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		wantKind inputErrorKind
	}{
		{
			name:     "rejects malformed JSON",
			body:     `{"title":`,
			wantKind: inputErrorMalformed,
		},
		{
			name: "rejects unknown fields",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":["10000000-0000-4000-8000-000000000001"],
				"html":"<script>alert(1)</script>"
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects explicit null optional string",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":["10000000-0000-4000-8000-000000000001"],
				"examName":null
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects trailing JSON",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":["10000000-0000-4000-8000-000000000001"]
			} {}`,
			wantKind: inputErrorMalformed,
		},
		{
			name: "rejects duplicate topics",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":[
					"10000000-0000-4000-8000-00000000000A",
					"10000000-0000-4000-8000-00000000000a"
				]
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects invalid topic UUID",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":["not-a-uuid"]
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects nil topic UUID",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":["00000000-0000-0000-0000-000000000000"]
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects numeric topic IDs",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":[1]
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects non HTTP image URL",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"file:///tmp/a.jpg",
				"topicIds":["10000000-0000-4000-8000-000000000001"]
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects blank required text",
			body: `{
				"title":" ", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":["10000000-0000-4000-8000-000000000001"]
			}`,
			wantKind: inputErrorValidation,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := decodeAndValidateCreatePost(strings.NewReader(tt.body))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if err.Kind != tt.wantKind {
				t.Fatalf("got error kind %q, want %q", err.Kind, tt.wantKind)
			}
		})
	}
}

func TestParseOptionalResourceID(t *testing.T) {
	t.Parallel()

	id, err := parseOptionalResourceID(
		" "+contentControllerUppercaseUUID+" ",
		"topicId",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != contentControllerCanonicalUUID {
		t.Fatalf("got %#v, want canonical UUID %q", id, contentControllerCanonicalUUID)
	}

	id, err = parseOptionalResourceID("", "topicId")
	if err != nil {
		t.Fatalf("unexpected error for absent value: %v", err)
	}
	if id != nil {
		t.Fatalf("got %#v, want nil", id)
	}

	for _, invalid := range []string{"42", "topic", contentControllerNilUUID} {
		if _, err = parseOptionalResourceID(invalid, "topicId"); err == nil ||
			err.Kind != inputErrorMalformed {
			t.Fatalf("parseOptionalResourceID(%q) error = %#v, want malformed", invalid, err)
		}
		if failure := inputFailure(err); failure.status != 400 {
			t.Fatalf(
				"parseOptionalResourceID(%q) HTTP status = %d, want 400",
				invalid,
				failure.status,
			)
		}
	}
}

func TestParseRequiredResourceID(t *testing.T) {
	t.Parallel()

	id, err := parseRequiredResourceID(contentControllerUppercaseUUID, "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != contentControllerCanonicalUUID {
		t.Fatalf("got %q, want canonical UUID %q", id, contentControllerCanonicalUUID)
	}

	for _, invalid := range []string{"", "25", contentControllerNilUUID} {
		if _, inputErr := parseRequiredResourceID(invalid, "id"); inputErr == nil ||
			inputErr.Kind != inputErrorMalformed {
			t.Fatalf("parseRequiredResourceID(%q) error = %#v, want malformed", invalid, inputErr)
		}
	}
}

func TestContentServiceFailureMapsPostOwnershipDenialToForbidden(t *testing.T) {
	t.Parallel()

	failure := contentServiceFailure(services.ErrForbidden)

	if failure.status != 403 {
		t.Fatalf("forbidden status = %d, want 403", failure.status)
	}
	if failure.code != "FORBIDDEN" {
		t.Fatalf("forbidden code = %q, want FORBIDDEN", failure.code)
	}
	if failure.message != "Bạn chỉ có thể xóa bài viết của chính mình" {
		t.Fatalf("forbidden message = %q", failure.message)
	}
}

func TestContentServiceFailureMapsInvalidDemoCredentialsToUnauthorized(t *testing.T) {
	t.Parallel()

	failure := contentServiceFailure(services.ErrInvalidDemoCredentials)

	if failure.status != 401 {
		t.Fatalf("invalid credentials status = %d, want 401", failure.status)
	}
	if failure.code != "INVALID_DEMO_CREDENTIALS" {
		t.Fatalf("invalid credentials code = %q", failure.code)
	}
	if failure.message != "Sai tài khoản hoặc mật khẩu demo" {
		t.Fatalf("invalid credentials message = %q", failure.message)
	}
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}
