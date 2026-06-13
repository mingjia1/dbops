package models

import (
	"time"
)

type AlertRule struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Metric             string    `json:"metric"`
	Condition          string    `json:"condition"`
	Threshold          float64   `json:"threshold"`
	DurationSeconds    int       `json:"duration_seconds"`
	Severity           string    `json:"severity"`
	NotificationChannels string   `json:"notification_channels"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	
	Records []AlertRecord `json:"records"`
}

type AlertRecord struct {
	ID          string     `json:"id"`
	RuleID      string     `json:"rule_id"`
	InstanceID  string     `json:"instance_id"`
	TriggeredAt time.Time  `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	Status      string     `json:"status"`
	Severity    string     `json:"severity"`
	Value       float64    `json:"value"`
	Message     string     `json:"message"`
	CreatedAt   time.Time  `json:"created_at"`
	
	Notifications []AlertNotification `json:"notifications"`
}

type AlertNotification struct {
	ID            string    `json:"id"`
	AlertID       string    `json:"alert_id"`
	ChannelType   string    `json:"channel_type"`
	ChannelConfig string    `json:"channel_config"`
	SentAt        time.Time `json:"sent_at"`
	Status        string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}