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
			name:     "removes a polite Vietnamese suffix from the topic",
			question: "Đếm lại chủ đề cà phê giúp mình.",
			want:     "ca phe",
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

func TestDetectAppService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		question string
		want     AssistantAppService
	}{
		{
			name:     "recognizes demo account help",
			question: "Mình đăng nhập Artly bằng tài khoản mẫu thế nào?",
			want:     AssistantAppServiceAccount,
		},
		{
			name:     "recognizes feed help",
			question: "Làm sao lọc chủ đề trên bảng tin?",
			want:     AssistantAppServiceFeed,
		},
		{
			name:     "recognizes post help",
			question: "Mình muốn đăng tác phẩm bằng URL ảnh.",
			want:     AssistantAppServicePosts,
		},
		{
			name:     "recognizes reaction help",
			question: "Làm sao bỏ tim một bài viết?",
			want:     AssistantAppServiceReactions,
		},
		{
			name:     "recognizes messaging help",
			question: "Mình nhắn tin cho bạn học ở đâu?",
			want:     AssistantAppServiceMessages,
		},
		{
			name:     "recognizes profile help",
			question: "Đổi tên hiển thị và avatar như thế nào?",
			want:     AssistantAppServiceProfile,
		},
		{
			name:     "recognizes assistant history help",
			question: "Mở lại lịch sử Trợ lý Artly ở đâu?",
			want:     AssistantAppServiceAssistant,
		},
		{
			name:     "recognizes general Artly service question",
			question: "Artly có những tính năng gì?",
			want:     AssistantAppServiceGeneral,
		},
		{
			name:     "recognizes a general question about Artly",
			question: "Artly là gì?",
			want:     AssistantAppServiceGeneral,
		},
		{
			name:     "leaves statistics questions to the count pipeline",
			question: "Có bao nhiêu bài viết về cà phê?",
			want:     "",
		},
		{
			name:     "does not confuse chemistry with reactions",
			question: "Phản ứng hóa học là gì?",
			want:     "",
		},
		{
			name:     "does not confuse essay writing with posts",
			question: "Giúp mình viết bài văn tả cảnh.",
			want:     "",
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := DetectAppService(testCase.question, nil)
			if got != testCase.want {
				t.Fatalf(
					"DetectAppService(%q) = %q, want %q",
					testCase.question,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestDetectAppServiceUsesLatestUserTurnForShortFollowUp(t *testing.T) {
	t.Parallel()

	history := []AssistantConversationMessage{
		{Role: "USER", Content: "Mình nhắn tin cho bạn học ở đâu?"},
		{Role: "ASSISTANT", Content: "Bạn mở mục Nhắn tin và chọn người muốn trò chuyện."},
	}

	got := DetectAppService("Còn gửi ảnh thì sao?", history)

	if got != AssistantAppServiceMessages {
		t.Fatalf("DetectAppService() = %q, want %q", got, AssistantAppServiceMessages)
	}
}

func TestDetectAppServiceDoesNotReuseHistoryForUnrelatedQuestion(t *testing.T) {
	t.Parallel()

	history := []AssistantConversationMessage{
		{Role: "USER", Content: "Mình nhắn tin cho bạn học ở đâu?"},
		{Role: "ASSISTANT", Content: "Bạn mở mục Nhắn tin và chọn người muốn trò chuyện."},
	}

	got := DetectAppService("Con người nhìn thấy màu sắc như thế nào?", history)

	if got != "" {
		t.Fatalf("DetectAppService() = %q, want no app service", got)
	}
}
