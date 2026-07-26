package models

import "encoding/json"

// AssistantConversation groups one user's persisted exchanges with Artly.
type AssistantConversation struct {
	BaseModel
	UserID string `gorm:"column:user_id;type:uuid;not null;index" db:"user_id" json:"userId"`
	Title  string `gorm:"column:title;type:varchar(80);not null" db:"title" json:"title"`

	Messages []AssistantMessage `gorm:"foreignKey:ConversationID;references:ID" json:"messages,omitempty"`
}

func (AssistantConversation) TableName() string {
	return "assistant_chat_conversations"
}

// AssistantMessage is an immutable user or assistant message in a conversation.
type AssistantMessage struct {
	BaseModel
	ConversationID string          `gorm:"column:conversation_id;type:uuid;not null;index" db:"conversation_id" json:"conversationId"`
	Position       int             `gorm:"column:position;not null" db:"position" json:"position"`
	Role           string          `gorm:"column:role;type:varchar(20);not null" db:"role" json:"role"`
	Content        string          `gorm:"column:content;type:text;not null" db:"content" json:"content"`
	ResponseJSON   json.RawMessage `gorm:"column:response_json;type:jsonb" db:"response_json" json:"-"`
}

func (AssistantMessage) TableName() string {
	return "assistant_chat_messages"
}
