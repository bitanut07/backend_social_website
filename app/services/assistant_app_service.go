package services

import (
	"strings"
	"unicode/utf8"
)

type AssistantAppService string

const (
	AssistantAppServiceGeneral   AssistantAppService = "GENERAL"
	AssistantAppServiceAccount   AssistantAppService = "ACCOUNT"
	AssistantAppServiceFeed      AssistantAppService = "FEED"
	AssistantAppServicePosts     AssistantAppService = "POSTS"
	AssistantAppServiceReactions AssistantAppService = "REACTIONS"
	AssistantAppServiceMessages  AssistantAppService = "MESSAGES"
	AssistantAppServiceProfile   AssistantAppService = "PROFILE"
	AssistantAppServiceAssistant AssistantAppService = "ASSISTANT"
)

func DetectAppService(
	question string,
	history []AssistantConversationMessage,
) AssistantAppService {
	normalized := NormalizeForSearch(question)
	if normalized == "" || isArtlyPostCountQuestion(normalized) {
		return ""
	}

	if service := detectDirectAppService(normalized); service != "" {
		return service
	}
	if !isShortAppServiceFollowUp(normalized) {
		return ""
	}

	for index := len(history) - 1; index >= 0; index-- {
		if history[index].Role != "USER" {
			continue
		}

		return detectDirectAppService(NormalizeForSearch(history[index].Content))
	}

	return ""
}

func detectDirectAppService(question string) AssistantAppService {
	switch {
	case containsAnyPhrase(
		question,
		"tro ly artly",
		"lich su tro chuyen",
		"lich su chat",
		"chat moi",
	):
		return AssistantAppServiceAssistant
	case containsAnyPhrase(
		question,
		"dang nhap",
		"dang xuat",
		"tai khoan mau",
		"tai khoan demo",
		"mat khau demo",
	):
		return AssistantAppServiceAccount
	case containsAnyPhrase(
		question,
		"tin nhan",
		"nhan tin",
		"gui tin",
		"hop thu",
	):
		return AssistantAppServiceMessages
	case containsAnyPhrase(
		question,
		"tha tim",
		"bo tim",
		"luot thich",
		"thich bai",
		"reaction",
	):
		return AssistantAppServiceReactions
	case containsAnyPhrase(
		question,
		"chinh sua profile",
		"chinh profile",
		"ho so ca nhan",
		"ten hien thi",
		"ten nguoi dung",
		"anh dai dien",
		"avatar",
		"username",
	):
		return AssistantAppServiceProfile
	case containsAnyPhrase(
		question,
		"dang bai",
		"dang tac pham",
		"tao bai",
		"xoa bai",
		"url anh",
	):
		return AssistantAppServicePosts
	case containsAnyPhrase(
		question,
		"bang tin",
		"loc chu de",
		"danh sach bai",
		"xem bai tren artly",
		"artly feed",
	):
		return AssistantAppServiceFeed
	case strings.Contains(question, "artly") &&
		containsAnyPhrase(
			question,
			"tinh nang",
			"dich vu",
			"su dung",
			"lam duoc gi",
			"ho tro gi",
		):
		return AssistantAppServiceGeneral
	case strings.Contains(question, "artly"):
		return AssistantAppServiceGeneral
	default:
		return ""
	}
}

func isArtlyPostCountQuestion(question string) bool {
	if !hasStatisticsIntent(question) {
		return false
	}

	return containsAnyPhrase(
		question,
		"bai viet",
		"bai dang",
		"tac pham",
		"chu de",
		"artly",
		"dem bai",
		"co may bai",
		"bao nhieu bai",
	)
}

func isShortAppServiceFollowUp(question string) bool {
	if utf8.RuneCountInString(question) > 120 {
		return false
	}

	return hasAnyPrefix(
		question,
		"con gui",
		"con lam",
		"con cach",
		"con neu",
		"con tinh nang",
		"the con",
		"vay con",
		"neu ",
		"gui anh",
		"lam tiep",
	)
}

func containsAnyPhrase(value string, phrases ...string) bool {
	for _, phrase := range phrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}

	return false
}

func hasAnyPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}

	return false
}

func appServiceHelpResponse(service AssistantAppService) AssistantResponse {
	return AssistantResponse{
		Status:     AssistantStatusAnswered,
		Intent:     AssistantIntentAppServiceHelp,
		Answer:     appServiceHelpAnswer(service),
		Provider:   AssistantProviderLocal,
		AppService: service,
	}
}

func appServiceHelpAnswer(service AssistantAppService) string {
	switch service {
	case AssistantAppServiceGeneral:
		return "Artly MVP có bảng tin và lọc chủ đề, đăng hoặc xóa tác phẩm bằng URL ảnh, reaction, hồ sơ cá nhân, nhắn tin văn bản và Trợ lý Artly có lịch sử cùng thống kê bài viết theo chủ đề."
	case AssistantAppServiceAccount:
		return "Artly MVP dùng tài khoản mẫu. Ở màn hình đăng nhập, nhập username và mật khẩu demo; sau khi vào ứng dụng, bạn có thể mở menu hồ sơ để đăng xuất."
	case AssistantAppServiceFeed:
		return "Mở Bảng tin để xem các tác phẩm mới nhất. Bạn có thể dùng bộ lọc chủ đề ở đầu danh sách để thu hẹp các bài đang hiển thị."
	case AssistantAppServicePosts:
		return "Trong Bảng tin, chọn Đăng bài, nhập tiêu đề, mô tả và URL ảnh bắt đầu bằng http:// hoặc https://. Bản REST hiện chưa tải file ảnh trực tiếp, và bạn chỉ có thể xóa bài do mình đăng."
	case AssistantAppServiceReactions:
		return "Chọn nút hình tim dưới một bài viết để thả reaction; chọn lại nút đó để gỡ reaction. Số lượt reaction được cập nhật ngay trên bài."
	case AssistantAppServiceMessages:
		return "Mở Nhắn tin, chọn một người dùng, nhập nội dung rồi gửi. Bản REST hiện hỗ trợ tin nhắn văn bản bằng polling; chưa hỗ trợ ảnh hoặc realtime."
	case AssistantAppServiceProfile:
		return "Mở menu tài khoản rồi vào hồ sơ để chỉnh tên hiển thị, username và URL avatar. Username phải đúng định dạng và không được trùng tài khoản khác."
	case AssistantAppServiceAssistant:
		return "Mở Trợ lý Artly để bắt đầu Chat mới hoặc chọn một cuộc trò chuyện cũ. Trợ lý lưu lịch sử theo tài khoản và có thể thống kê số bài viết theo chủ đề."
	default:
		return ""
	}
}
