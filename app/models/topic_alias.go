package models

// TopicAlias maps an alternative spelling to exactly one canonical topic.
type TopicAlias struct {
	BaseModel
	TopicID         string `gorm:"column:topic_id;type:uuid;not null;index" db:"topic_id" json:"topicId"`
	Alias           string `gorm:"column:alias;size:100;not null" db:"alias" json:"alias"`
	NormalizedAlias string `gorm:"column:normalized_alias;size:100;not null;uniqueIndex" db:"normalized_alias" json:"normalizedAlias"`
}

func (TopicAlias) TableName() string {
	return "topic_aliases"
}
