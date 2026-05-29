package models

import (
	"time"
)

type AlertRule struct {
	ID                 string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name               string    `json:"name" gorm:"type:varchar(128);not null"`
	Metric             string    `json:"metric" gorm:"type:varchar(128);not null"`
	Condition          string    `json:"condition" gorm:"type:varchar(32);not null"`
	Threshold          float64   `json:"threshold" gorm:"type:decimal(10,2);not null"`
	DurationSeconds    int       `json:"duration_seconds" gorm:"type:int;default:60"`
	Severity           string    `json:"severity" gorm:"type:varchar(16);not null"`
	NotificationChannels string   `json:"notification_channels" gorm:"type:text"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	
	Records []AlertRecord `json:"records" gorm:"foreignKey:RuleID"`
}

type AlertRecord struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RuleID      string     `json:"rule_id" gorm:"type:varchar(64);index"`
	InstanceID  string     `json:"instance_id" gorm:"type:varchar(64);index"`
	TriggeredAt time.Time  `json:"triggered_at" gorm:"type:timestamp;not null"`
	ResolvedAt  *time.Time `json:"resolved_at" gorm:"type:timestamp"`
	Status      string     `json:"status" gorm:"type:varchar(32);default:'firing'"`
	Severity    string     `json:"severity" gorm:"type:varchar(16);not null"`
	Value       float64    `json:"value" gorm:"type:decimal(10,2)"`
	Message     string     `json:"message" gorm:"type:text"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	
	Notifications []AlertNotification `json:"notifications" gorm:"foreignKey:AlertID"`
}

type AlertNotification struct {
	ID            string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	AlertID       string    `json:"alert_id" gorm:"type:varchar(64);index"`
	ChannelType   string    `json:"channel_type" gorm:"type:varchar(32);not null"`
	ChannelConfig string    `json:"channel_config" gorm:"type:text"`
	SentAt        time.Time `json:"sent_at" gorm:"type:timestamp"`
	Status        string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}