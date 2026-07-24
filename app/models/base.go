package models

import "github.com/goravel/framework/support/carbon"

// Timestamps mirrors the created_at and updated_at columns shared by Artly tables.
type Timestamps struct {
	CreatedAt *carbon.DateTime `gorm:"autoCreateTime;column:created_at" db:"created_at" json:"createdAt"`
	UpdatedAt *carbon.DateTime `gorm:"autoUpdateTime;column:updated_at" db:"updated_at" json:"updatedAt"`
}

// BaseModel contains the numeric primary key used by the public API.
type BaseModel struct {
	ID uint64 `gorm:"column:id;primaryKey;autoIncrement" db:"id" json:"id"`
	Timestamps
}
