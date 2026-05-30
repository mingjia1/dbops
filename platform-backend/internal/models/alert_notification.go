package models

import (
	"time"
)

type NotificationChannel struct {
	ID            string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name          string    `json:"name" gorm:"type:varchar(128);uniqueIndex;not null"`
	ChannelType   string    `json:"channel_type" gorm:"type:varchar(32);not null"`
	ChannelConfig string    `json:"channel_config" gorm:"type:text"`
	Template      string    `json:"template" gorm:"type:text"`
	IsActive      bool      `json:"is_active" gorm:"type:boolean;default:true;index"`
	Priority      int       `json:"priority" gorm:"type:int;default:1"`
	Description   string    `json:"description" gorm:"type:text"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (nc *NotificationChannel) ToAlertNotification(alertID string) AlertNotification {
	return AlertNotification{
		AlertID:       alertID,
		ChannelType:   nc.ChannelType,
		ChannelConfig: nc.ChannelConfig,
		Status:        "pending",
	}
}