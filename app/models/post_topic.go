package models

// PostTopic is the explicit many-to-many join model for posts and topics.
type PostTopic struct {
	PostID  string `gorm:"column:post_id;type:uuid;primaryKey" db:"post_id" json:"postId"`
	TopicID string `gorm:"column:topic_id;type:uuid;primaryKey" db:"topic_id" json:"topicId"`
	Timestamps
}

func (PostTopic) TableName() string {
	return "post_topics"
}
