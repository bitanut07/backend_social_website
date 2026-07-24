package controllers

import (
	"strings"
	"testing"

	"github.com/goravel/framework/contracts/http"
)

const (
	messageControllerTestUserOneID = "00000000-0000-4000-8000-000000000001"
	messageControllerTestUserTwoID = "00000000-0000-4000-8000-000000000002"
	messageControllerTestNilUUID   = "00000000-0000-0000-0000-000000000000"
)

func TestParseMessageListParamsUsesContractDefaults(t *testing.T) {
	t.Parallel()

	params, failure := parseMessageListParams(map[string]string{
		"peerId": messageControllerTestUserTwoID,
	})

	if failure != nil {
		t.Fatalf("unexpected failure: %#v", failure)
	}
	if params.PeerID != messageControllerTestUserTwoID || params.Page != 1 || params.PageSize != 50 {
		t.Fatalf("unexpected parameters: %#v", params)
	}
}

func TestParseMessageListParamsDistinguishesMalformedAndInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		query      map[string]string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing peer is a validation error",
			query:      map[string]string{},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "text peer is a bad request",
			query:      map[string]string{"peerId": "teacher"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:       "nil UUID peer is a bad request",
			query:      map[string]string{"peerId": messageControllerTestNilUUID},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:       "text page is a bad request",
			query:      map[string]string{"peerId": messageControllerTestUserTwoID, "page": "first"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:       "zero page is a validation error",
			query:      map[string]string{"peerId": messageControllerTestUserTwoID, "page": "0"},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "page size over one hundred is a validation error",
			query:      map[string]string{"peerId": messageControllerTestUserTwoID, "pageSize": "101"},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "page above global maximum is a validation error",
			query:      map[string]string{"peerId": messageControllerTestUserTwoID, "page": "100001"},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, failure := parseMessageListParams(tt.query)
			if failure == nil {
				t.Fatal("expected request failure")
			}
			if failure.status != tt.wantStatus || failure.code != tt.wantCode {
				t.Fatalf("got status/code %d/%s, want %d/%s", failure.status, failure.code, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestDecodeCreateMessageRequestRejectsUnknownAndTrailingJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{
			name: "unknown property",
			body: `{"recipientId":"00000000-0000-4000-8000-000000000002","body":"Xin chào","html":"<b>bad</b>"}`,
		},
		{
			name: "trailing JSON value",
			body: `{"recipientId":"00000000-0000-4000-8000-000000000002","body":"Xin chào"} {}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := decodeCreateMessageRequest(strings.NewReader(tt.body)); err == nil {
				t.Fatal("expected malformed request error")
			}
		})
	}
}

func TestMessageJSONFailureDistinguishesSchemaAndSyntaxErrors(t *testing.T) {
	t.Parallel()

	_, schemaErr := decodeCreateMessageRequest(strings.NewReader(
		`{"recipientId":2,"body":"Xin chào"}`,
	))
	if schemaErr == nil {
		t.Fatal("expected schema error")
	}
	schemaFailure := messageJSONFailure(schemaErr)
	if schemaFailure.status != http.StatusUnprocessableEntity || schemaFailure.code != "VALIDATION_ERROR" {
		t.Fatalf("unexpected schema failure: %#v", schemaFailure)
	}

	_, syntaxErr := decodeCreateMessageRequest(strings.NewReader(
		`{"recipientId":"00000000-0000-4000-8000-000000000002","body":`,
	))
	if syntaxErr == nil {
		t.Fatal("expected syntax error")
	}
	syntaxFailure := messageJSONFailure(syntaxErr)
	if syntaxFailure.status != http.StatusBadRequest || syntaxFailure.code != "BAD_REQUEST" {
		t.Fatalf("unexpected syntax failure: %#v", syntaxFailure)
	}
}

func TestValidateCreateMessageRequestTrimsBodyAndCountsUnicodeCharacters(t *testing.T) {
	t.Parallel()

	recipientID := messageControllerTestUserTwoID
	body := "  Cô xem giúp em ạ.  "
	input := createMessageRequest{RecipientID: &recipientID, Body: &body}

	validated, failure := validateCreateMessageRequest(input)

	if failure != nil {
		t.Fatalf("unexpected failure: %#v", failure)
	}
	if validated.Body != "Cô xem giúp em ạ." ||
		validated.RecipientID != messageControllerTestUserTwoID {
		t.Fatalf("unexpected validated request: %#v", validated)
	}

	twoThousandVietnameseCharacters := strings.Repeat("ộ", 2000)
	input.Body = &twoThousandVietnameseCharacters
	if _, failure := validateCreateMessageRequest(input); failure != nil {
		t.Fatalf("2000 Unicode characters should be valid: %#v", failure)
	}

	twoThousandAndOneVietnameseCharacters := strings.Repeat("ộ", 2001)
	input.Body = &twoThousandAndOneVietnameseCharacters
	if _, failure := validateCreateMessageRequest(input); failure == nil {
		t.Fatal("2001 Unicode characters must be rejected")
	}
}

func TestValidateCreateMessageRequestRejectsMissingFields(t *testing.T) {
	t.Parallel()

	if _, failure := validateCreateMessageRequest(createMessageRequest{}); failure == nil {
		t.Fatal("missing recipientId and body must be rejected")
	}
}

func TestValidateCreateMessageRequestRejectsInvalidAndNilRecipientUUIDs(t *testing.T) {
	t.Parallel()

	body := "Xin chào"
	for _, recipientID := range []string{"teacher", messageControllerTestNilUUID} {
		recipientID := recipientID
		t.Run(recipientID, func(t *testing.T) {
			t.Parallel()

			input := createMessageRequest{RecipientID: &recipientID, Body: &body}
			_, failure := validateCreateMessageRequest(input)
			if failure == nil {
				t.Fatal("invalid recipient UUID must be rejected")
			}
			if failure.status != http.StatusUnprocessableEntity ||
				failure.code != "VALIDATION_ERROR" {
				t.Fatalf("unexpected failure: %#v", failure)
			}
		})
	}
}

func TestValidateCreateMessageRequestLeavesParticipantRulesToTheService(t *testing.T) {
	t.Parallel()

	recipientID := messageControllerTestUserOneID
	body := "Nội dung hợp lệ về cấu trúc"
	input := createMessageRequest{RecipientID: &recipientID, Body: &body}

	validated, failure := validateCreateMessageRequest(input)

	if failure != nil {
		t.Fatalf("participant rule should be evaluated after current-user existence: %#v", failure)
	}
	if validated.RecipientID != messageControllerTestUserOneID {
		t.Fatalf("unexpected recipient: %#v", validated)
	}
}
