package support

import "testing"

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
