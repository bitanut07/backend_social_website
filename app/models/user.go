package models

// User is a demo identity selected through the X-User-ID header.
type User struct {
	BaseModel
	Username    string  `gorm:"column:username;size:50;not null;uniqueIndex" db:"username" json:"username"`
	DisplayName string  `gorm:"column:display_name;size:100;not null" db:"display_name" json:"displayName"`
	Role        string  `gorm:"column:role;size:20;not null" db:"role" json:"role"`
	AvatarURL   *string `gorm:"column:avatar_url;size:2048" db:"avatar_url" json:"avatarUrl"`
}

func (User) TableName() string {
	return "users"
}
