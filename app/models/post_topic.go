package models

// PostTopic is the explicit many-to-many join model for posts and topics.
type PostTopic struct {
	BaseModel
	PostID  string `gorm:"column:post_id;type:uuid;not null;uniqueIndex:post_topics_pair_unique" db:"post_id" json:"postId"`
	TopicID string `gorm:"column:topic_id;type:uuid;not null;uniqueIndex:post_topics_pair_unique" db:"topic_id" json:"topicId"`
}

func (PostTopic) TableName() string {
	return "post_topics"
}
