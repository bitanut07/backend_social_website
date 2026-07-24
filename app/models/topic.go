package models

// Topic is the canonical subject used to classify posts and answer statistics questions.
type Topic struct {
	BaseModel
	Slug           string       `gorm:"column:slug;size:100;not null;uniqueIndex" db:"slug" json:"slug"`
	Name           string       `gorm:"column:name;size:100;not null" db:"name" json:"name"`
	NormalizedName string       `gorm:"column:normalized_name;size:100;not null;uniqueIndex" db:"normalized_name" json:"normalizedName"`
	Aliases        []TopicAlias `gorm:"foreignKey:TopicID;references:ID" json:"aliases,omitempty"`
}

func (Topic) TableName() string {
	return "topics"
}
