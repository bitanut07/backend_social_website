package services

import (
	"strings"
	"testing"
)

func TestNormalizeForSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercases and removes Vietnamese diacritics",
			input: "Đếm BÀI VỀ Hòa Bình",
			want:  "dem bai ve hoa binh",
		},
		{
			name:  "trims collapses whitespace and replaces punctuation",
			input: " \t Bảo vệ,\n môi-trường!!!  ",
			want:  "bao ve moi truong",
		},
		{
			name:  "returns empty for whitespace and punctuation only",
			input: "  ...?!  ",
			want:  "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeForSearch(tt.input); got != tt.want {
				t.Fatalf("NormalizeForSearch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractTopicCandidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		question string
		want     string
	}{
		{
			name:     "extracts topic after subject phrase",
			question: "Có bao nhiêu bài nói về chủ đề bảo vệ môi trường?",
			want:     "bao ve moi truong",
		},
		{
			name:     "prefers quoted topic",
			question: `Đếm bài chủ đề "Hòa bình"`,
			want:     "hoa binh",
		},
		{
			name:     "extracts topic after about phrase",
			question: "Có mấy tác phẩm về di sản quê em?",
			want:     "di san que em",
		},
		{
			name:     "trims topic and trailing punctuation",
			question: "  Đếm bài về: biến đổi khí hậu!!!  ",
			want:     "bien doi khi hau",
		},
		{
			name:     "does not treat a greeting as a statistics question",
			question: "Xin chào, hôm nay bạn khỏe không?",
			want:     "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ExtractTopicCandidate(tt.question); got != tt.want {
				t.Fatalf("ExtractTopicCandidate(%q) = %q, want %q", tt.question, got, tt.want)
			}
		})
	}
}

func TestExtractTopicCandidateLimitsNormalizedTopicToOneHundredCharacters(t *testing.T) {
	t.Parallel()

	longTopic := strings.Repeat("a", 140)
	question := `Đếm bài chủ đề "` + longTopic + `"`
	want := strings.Repeat("a", 100)

	if got := ExtractTopicCandidate(question); got != want {
		t.Fatalf("ExtractTopicCandidate() returned %d characters, want a normalized maximum of %d", len(got), len(want))
	}
}
