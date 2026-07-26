package seeders

import (
	"fmt"
	"strings"
)

const cafeDemoTopicID = "20000000-0000-4000-8000-000000000001"

type cafeDemoUser struct {
	Email       string
	Username    string
	DisplayName string
	Role        string
	Bio         string
}

type cafeDemoTopic struct {
	ID          string
	Slug        string
	Name        string
	Normalized  string
	Description string
}

type cafeDemoPost struct {
	ID            string
	AuthorEmail   string
	Title         string
	Caption       string
	ExamName      string
	ImageFile     string
	ImageURL      string
	PublishedAgoH int
}

type cafeDemoReaction struct {
	PostID         string
	ReactorEmail   string
	ReactionTypeID string
}

type cafeDemoDataset struct {
	Users     []cafeDemoUser
	Topic     cafeDemoTopic
	Posts     []cafeDemoPost
	Reactions []cafeDemoReaction
}

func cafeDemoData(assetBaseURL string) cafeDemoDataset {
	baseURL := strings.TrimRight(strings.TrimSpace(assetBaseURL), "/")
	imageURL := func(fileName string) string {
		return fmt.Sprintf("%s/%s", baseURL, fileName)
	}

	return cafeDemoDataset{
		Users: []cafeDemoUser{
			{
				Email:       "demo.thuha@artly.example",
				Username:    "thu.ha.cafe",
				DisplayName: "Lê Thu Hà",
				Role:        "STUDENT",
				Bio:         "Thích ký họa những góc quán nhỏ và ngày mưa.",
			},
			{
				Email:       "demo.quanghuy@artly.example",
				Username:    "quang.huy.art",
				DisplayName: "Nguyễn Quang Huy",
				Role:        "STUDENT",
				Bio:         "Mê màu gouache, cà phê phin và những họa tiết Sài Gòn.",
			},
			{
				Email:       "demo.mylinh@artly.example",
				Username:    "my.linh.sketch",
				DisplayName: "Trần Mỹ Linh",
				Role:        "STUDENT",
				Bio:         "Vẽ con người, những cuộc gặp gỡ và không gian sáng tạo.",
			},
			{
				Email:       "demo.thaynam@artly.example",
				Username:    "thay.nam.coffee",
				DisplayName: "Thầy Võ Hoàng Nam",
				Role:        "TEACHER",
				Bio:         "Giáo viên mỹ thuật, yêu câu chuyện từ nông trại đến ly cà phê.",
			},
		},
		Topic: cafeDemoTopic{
			ID:         cafeDemoTopicID,
			Slug:       "ca-phe",
			Name:       "Cà phê",
			Normalized: "ca-phe",
			Description: "Tranh và tác phẩm lấy cảm hứng từ cà phê, quán xá " +
				"và văn hóa thưởng thức.",
		},
		Posts: []cafeDemoPost{
			{
				ID:          "26000000-0000-4000-8000-000000000001",
				AuthorEmail: "demo.thuha@artly.example",
				Title:       "Góc cà phê ngày mưa",
				Caption: "Mình thử giữ lại cảm giác ấm áp của một góc cà phê " +
					"nhìn qua ô cửa đầy hạt mưa, nơi cuốn sổ ký họa luôn mở sẵn.",
				ExamName:      "Khoảnh khắc quán quen 2026",
				ImageFile:     "ca-phe-ngay-mua.webp",
				ImageURL:      imageURL("ca-phe-ngay-mua.webp"),
				PublishedAgoH: 1,
			},
			{
				ID:          "26000000-0000-4000-8000-000000000002",
				AuthorEmail: "demo.quanghuy@artly.example",
				Title:       "Cà phê phin buổi sáng",
				Caption: "Ly cà phê sữa đá, chiếc phin bạc và nền gạch cũ là " +
					"những điều rất Việt mà mình muốn kể bằng màu gouache.",
				ExamName:      "Hương vị Việt qua nét vẽ",
				ImageFile:     "ca-phe-phin-buoi-sang.webp",
				ImageURL:      imageURL("ca-phe-phin-buoi-sang.webp"),
				PublishedAgoH: 3,
			},
			{
				ID:          "26000000-0000-4000-8000-000000000003",
				AuthorEmail: "demo.mylinh@artly.example",
				Title:       "Hẹn nhau vẽ ở quán cà phê",
				Caption: "Một buổi học vẽ vui hơn khi có mùi cà phê, tiếng trò " +
					"chuyện và chiếc lá latte art vừa được hoàn thành.",
				ExamName:      "Sáng tạo cùng cà phê",
				ImageFile:     "goc-ve-cung-barista.webp",
				ImageURL:      imageURL("goc-ve-cung-barista.webp"),
				PublishedAgoH: 6,
			},
			{
				ID:          "26000000-0000-4000-8000-000000000004",
				AuthorEmail: "demo.thaynam@artly.example",
				Title:       "Hành trình của hạt cà phê",
				Caption: "Từ những quả cà phê đỏ trên cao nguyên đến chiếc cốc " +
					"ấm giữa thành phố, mỗi công đoạn đều có một câu chuyện riêng.",
				ExamName:      "Hành trình hạt cà phê",
				ImageFile:     "hanh-trinh-hat-ca-phe.webp",
				ImageURL:      imageURL("hanh-trinh-hat-ca-phe.webp"),
				PublishedAgoH: 9,
			},
		},
		Reactions: []cafeDemoReaction{
			{
				PostID:         "26000000-0000-4000-8000-000000000001",
				ReactorEmail:   "demo.quanghuy@artly.example",
				ReactionTypeID: "10000000-0000-4000-8000-000000000002",
			},
			{
				PostID:         "26000000-0000-4000-8000-000000000001",
				ReactorEmail:   "demo.mylinh@artly.example",
				ReactionTypeID: "10000000-0000-4000-8000-000000000007",
			},
			{
				PostID:         "26000000-0000-4000-8000-000000000002",
				ReactorEmail:   "demo.thuha@artly.example",
				ReactionTypeID: "10000000-0000-4000-8000-000000000001",
			},
			{
				PostID:         "26000000-0000-4000-8000-000000000002",
				ReactorEmail:   "demo.thaynam@artly.example",
				ReactionTypeID: "10000000-0000-4000-8000-000000000007",
			},
			{
				PostID:         "26000000-0000-4000-8000-000000000003",
				ReactorEmail:   "demo.thaynam@artly.example",
				ReactionTypeID: "10000000-0000-4000-8000-000000000002",
			},
			{
				PostID:         "26000000-0000-4000-8000-000000000003",
				ReactorEmail:   "demo.thuha@artly.example",
				ReactionTypeID: "10000000-0000-4000-8000-000000000007",
			},
			{
				PostID:         "26000000-0000-4000-8000-000000000004",
				ReactorEmail:   "demo.mylinh@artly.example",
				ReactionTypeID: "10000000-0000-4000-8000-000000000001",
			},
			{
				PostID:         "26000000-0000-4000-8000-000000000004",
				ReactorEmail:   "demo.quanghuy@artly.example",
				ReactionTypeID: "10000000-0000-4000-8000-000000000002",
			},
		},
	}
}
