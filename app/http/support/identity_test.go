package support

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	return function(request)
}

func TestParseUserIDHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:  "accepts a UUID",
			value: "00000000-0000-4000-8000-000000000002",
			want:  "00000000-0000-4000-8000-000000000002",
		},
		{
			name:  "trims spaces and normalizes UUID casing",
			value: " 00000000-0000-4000-8000-00000000000A ",
			want:  "00000000-0000-4000-8000-00000000000a",
		},
		{name: "rejects empty header", value: "", wantErr: true},
		{
			name:    "rejects nil UUID",
			value:   "00000000-0000-0000-0000-000000000000",
			wantErr: true,
		},
		{name: "rejects numeric id", value: "2", wantErr: true},
		{name: "rejects text", value: "student", wantErr: true},
		{
			name:    "rejects compact UUID",
			value:   "00000000000040008000000000000002",
			wantErr: true,
		},
		{
			name:    "rejects UUID URN",
			value:   "urn:uuid:00000000-0000-4000-8000-000000000002",
			wantErr: true,
		},
		{
			name:    "rejects braced UUID",
			value:   "{00000000-0000-4000-8000-000000000002}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseUserIDHeader(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got id %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseResourceIDRejectsInvalidUUID(t *testing.T) {
	t.Parallel()

	if _, err := ParseResourceID("42"); err == nil {
		t.Fatal("numeric resource ID must be rejected")
	}
}

func TestParseBearerToken(t *testing.T) {
	t.Parallel()

	token, err := ParseBearerToken("  Bearer access-token  ")
	if err != nil || token != "access-token" {
		t.Fatalf("got token %q and error %v", token, err)
	}

	for _, value := range []string{"", "Basic token", "Bearer", "Bearer a b"} {
		if _, err := ParseBearerToken(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestVerifySupabaseAccessToken(t *testing.T) {
	t.Parallel()

	const expectedID = "00000000-0000-4000-8000-00000000000a"
	client := &http.Client{Transport: roundTripperFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		if request.URL.Path != "/auth/v1/user" {
			t.Errorf("unexpected path %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer valid-token" {
			t.Errorf("missing bearer token")
		}
		if request.Header.Get("apikey") != "publishable-key" {
			t.Errorf("missing publishable key")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"` + expectedID + `"}`)),
		}, nil
	})}

	got, err := VerifySupabaseAccessToken(
		"valid-token",
		"https://project.supabase.co",
		"publishable-key",
		client,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expectedID {
		t.Fatalf("got %q, want %q", got, expectedID)
	}
}

func TestVerifySupabaseAccessTokenRejectsUnauthorized(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(
		_ *http.Request,
	) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"expired"}`)),
		}, nil
	})}

	if _, err := VerifySupabaseAccessToken(
		"expired-token",
		"https://project.supabase.co",
		"publishable-key",
		client,
	); err == nil {
		t.Fatal("unauthorized token must be rejected")
	}
}
