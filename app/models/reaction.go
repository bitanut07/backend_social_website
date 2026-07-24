package models

// Reaction stores one current reaction for each post and user pair.
type Reaction struct {
	BaseModel
	PostID uint64 `gorm:"column:post_id;not null;uniqueIndex:reactions_post_user_unique" db:"post_id" json:"postId"`
	UserID uint64 `gorm:"column:user_id;not null;uniqueIndex:reactions_post_user_unique" db:"user_id" json:"userId"`
	Type   string `gorm:"column:type;size:20;not null;default:LIKE" db:"type" json:"type"`
}

func (Reaction) TableName() string {
	return "reactions"
}
