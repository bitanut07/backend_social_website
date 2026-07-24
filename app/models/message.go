package models

// Message is a direct, plain-text message between two demo users.
type Message struct {
	BaseModel
	SenderID   uint64 `gorm:"column:sender_id;not null;index" db:"sender_id" json:"senderId"`
	ReceiverID uint64 `gorm:"column:receiver_id;not null;index" db:"receiver_id" json:"receiverId"`
	Body       string `gorm:"column:body;type:text;not null" db:"body" json:"body"`
	IsRead     bool   `gorm:"column:is_read;not null;default:false;index" db:"is_read" json:"isRead"`

	Sender   User `gorm:"foreignKey:SenderID;references:ID" json:"sender,omitempty"`
	Receiver User `gorm:"foreignKey:ReceiverID;references:ID" json:"receiver,omitempty"`
}

func (Message) TableName() string {
	return "messages"
}
