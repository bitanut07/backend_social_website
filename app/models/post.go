package models

// Post is an artwork or drawing-exam submission shown in the feed.
type Post struct {
	BaseModel
	UserID   string  `gorm:"column:user_id;type:uuid;not null;index" db:"user_id" json:"userId"`
	Title    string  `gorm:"column:title;size:120;not null" db:"title" json:"title"`
	Caption  string  `gorm:"column:caption;type:text;not null" db:"caption" json:"caption"`
	ImageURL string  `gorm:"column:image_url;size:2048;not null" db:"image_url" json:"imageUrl"`
	ExamName *string `gorm:"column:exam_name;size:160" db:"exam_name" json:"examName,omitempty"`
	Status   string  `gorm:"column:status;size:20;not null;default:PUBLISHED;index" db:"status" json:"status"`

	Author    User       `gorm:"foreignKey:UserID;references:ID" json:"author,omitempty"`
	Topics    []Topic    `gorm:"many2many:post_topics;joinForeignKey:PostID;joinReferences:TopicID" json:"topics,omitempty"`
	Reactions []Reaction `gorm:"foreignKey:PostID;references:ID" json:"-"`
}

func (Post) TableName() string {
	return "posts"
}
