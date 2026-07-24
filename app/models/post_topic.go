package models

// PostTopic is the explicit many-to-many join model for posts and topics.
type PostTopic struct {
	PostID  uint64 `gorm:"column:post_id;primaryKey" db:"post_id" json:"postId"`
	TopicID uint64 `gorm:"column:topic_id;primaryKey" db:"topic_id" json:"topicId"`
	Timestamps
}

func (PostTopic) TableName() string {
	return "post_topics"
}
