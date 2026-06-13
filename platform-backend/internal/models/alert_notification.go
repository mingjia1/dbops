package models

import (
	"time"
)

type NotificationChannel struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ChannelType   string    `json:"channel_type"`
	ChannelConfig string    `json:"channel_config"`
	Template      string    `json:"template"`
	IsActive      bool      `json:"is_active"`
	Priority      int       `json:"priority"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (nc *NotificationChannel) ToAlertNotification(alertID string) AlertNotification {
	return AlertNotification{
		AlertID:       alertID,
		ChannelType:   nc.ChannelType,
		ChannelConfig: nc.ChannelConfig,
		Status:        "pending",
	}
}