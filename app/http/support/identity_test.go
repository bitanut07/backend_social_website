package support

import "testing"

func TestParseUserIDHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{name: "accepts a positive id", value: "2", want: 2},
		{name: "trims surrounding spaces", value: " 3 ", want: 3},
		{name: "rejects empty header", value: "", wantErr: true},
		{name: "rejects zero", value: "0", wantErr: true},
		{name: "rejects text", value: "student", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseUserIDHeader(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got id %d", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
