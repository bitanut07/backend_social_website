package controllers

import (
	"strings"
	"testing"
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
		"topicIds": [2, 4]
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
	if len(request.TopicIDs) != 2 || request.TopicIDs[0] != 2 || request.TopicIDs[1] != 4 {
		t.Fatalf("unexpected topic ids: %#v", request.TopicIDs)
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
				"topicIds":[1], "html":"<script>alert(1)</script>"
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects explicit null optional string",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":[1], "examName":null
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects trailing JSON",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":[1]
			} {}`,
			wantKind: inputErrorMalformed,
		},
		{
			name: "rejects duplicate topics",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":[1,1]
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects non HTTP image URL",
			body: `{
				"title":"T", "caption":"C", "imageUrl":"file:///tmp/a.jpg",
				"topicIds":[1]
			}`,
			wantKind: inputErrorValidation,
		},
		{
			name: "rejects blank required text",
			body: `{
				"title":" ", "caption":"C", "imageUrl":"https://example.com/a.jpg",
				"topicIds":[1]
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

	id, err := parseOptionalResourceID(" 42 ", "topicId")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != 42 {
		t.Fatalf("got %#v, want 42", id)
	}

	id, err = parseOptionalResourceID("", "topicId")
	if err != nil {
		t.Fatalf("unexpected error for absent value: %v", err)
	}
	if id != nil {
		t.Fatalf("got %#v, want nil", id)
	}

	if _, err = parseOptionalResourceID("-1", "topicId"); err == nil || err.Kind != inputErrorValidation {
		t.Fatalf("got %#v, want validation error", err)
	}
	if _, err = parseOptionalResourceID("topic", "topicId"); err == nil || err.Kind != inputErrorMalformed {
		t.Fatalf("got %#v, want malformed error", err)
	}
}
