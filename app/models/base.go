package models

import "github.com/goravel/framework/support/carbon"

// Timestamps mirrors the created_at and updated_at columns shared by Artly tables.
type Timestamps struct {
	CreatedAt *carbon.DateTime `gorm:"autoCreateTime;column:created_at" db:"created_at" json:"createdAt"`
	UpdatedAt *carbon.DateTime `gorm:"autoUpdateTime;column:updated_at" db:"updated_at" json:"updatedAt"`
}

// BaseModel contains the UUID primary key used by the public API.
type BaseModel struct {
	ID string `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()" db:"id" json:"id"`
	Timestamps
}
