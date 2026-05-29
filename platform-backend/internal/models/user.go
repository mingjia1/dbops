package models

import (
	"time"
)

type User struct {
	ID       string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Username string    `json:"username" gorm:"type:varchar(64);uniqueIndex;not null"`
	Password string    `json:"-" gorm:"type:varchar(255);not null"`
	Email    string    `json:"email" gorm:"type:varchar(128);uniqueIndex"`
	Role     string    `json:"role" gorm:"type:varchar(32);not null;default:'operator'"`
	Status   string    `json:"status" gorm:"type:varchar(16);default:'active'"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

type Role struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name        string    `json:"name" gorm:"type:varchar(32);uniqueIndex;not null"`
	Permissions string    `json:"permissions" gorm:"type:text"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}