package models

// Message is a direct, plain-text message stored in a conversation.
type Message struct {
	BaseModel
	ConversationID string `gorm:"column:conversation_id;type:uuid;not null;index" db:"conversation_id" json:"conversationId"`
	SenderID       string `gorm:"column:sender_id;type:uuid;index" db:"sender_id" json:"senderId"`
	Body           string `gorm:"column:body;type:text" db:"body" json:"body"`

	// Receiver is derived from the other participant in a DIRECT conversation.
	ReceiverID string `gorm:"-" db:"-" json:"receiverId"`
	IsRead     bool   `gorm:"-" db:"-" json:"isRead"`
	Sender     User   `gorm:"foreignKey:SenderID;references:ID" json:"sender,omitempty"`
	Receiver   User   `gorm:"-" json:"receiver,omitempty"`
}

func (Message) TableName() string {
	return "messages"
}
