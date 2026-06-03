package models

import "time"

type Host struct {
	ID                string     `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name              string     `json:"name" gorm:"type:varchar(255);not null;uniqueIndex"`
	Address           string     `json:"address" gorm:"type:varchar(255);not null"`
	SSHPort           int        `json:"ssh_port" gorm:"type:int;default:22"`
	SSHUser           string     `json:"ssh_user" gorm:"type:varchar(64);not null"`
	SSHAuthMethod     string     `json:"ssh_auth_method" gorm:"type:varchar(16);default:'password'"`
	SSHCredential     string     `json:"-" gorm:"type:text"`
	OSType            string     `json:"os_type" gorm:"type:varchar(32);default:'linux'"`
	Description       string     `json:"description" gorm:"type:text"`
	Tags              string     `json:"tags" gorm:"type:varchar(512)"`
	Status            string     `json:"status" gorm:"type:varchar(32);default:'unknown'"`
	LastCheckAt       *time.Time `json:"last_check_at" gorm:"type:timestamp"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}
