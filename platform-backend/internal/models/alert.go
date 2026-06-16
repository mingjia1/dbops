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
	Expression         string    `json:"expression"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	
	Records []AlertRecord `json:"records"`
}

type AlertTemplate struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Metric    string    `json:"metric"`
	Condition string    `json:"condition"`
	Threshold float64   `json:"threshold"`
	Expression string   `json:"expression"`
	DurationSeconds int `json:"duration_seconds"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type AlertEscalation struct {
	ID            string    `json:"id"`
	RuleID        string    `json:"rule_id"`
	Level         int       `json:"level"`
	Severity      string    `json:"severity"`
	IntervalSec   int       `json:"interval_sec"`
	NotifyChannels string   `json:"notify_channels"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AlertSilence struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	RuleIDs    string    `json:"rule_ids"`
	MatchExpr  string    `json:"match_expr"`
	StartAt    time.Time `json:"start_at"`
	EndAt      time.Time `json:"end_at"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type InspectionTemplate struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Config    string    `json:"config"`
	Schedule  string    `json:"schedule"`
	Recipients string   `json:"recipients"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type InspectionReport struct {
	ID           string     `json:"id"`
	TemplateID   string     `json:"template_id"`
	InstanceID   string     `json:"instance_id"`
	Status       string     `json:"status"`
	Summary      string     `json:"summary"`
	Details      string     `json:"details"`
	Score        int        `json:"score"`
	GeneratedAt  time.Time  `json:"generated_at"`
	SentAt       *time.Time `json:"sent_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type FaultTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	FaultType   string    `json:"fault_type"`
	Params      string    `json:"params"`
	DurationSec int       `json:"duration_sec"`
	Severity    string    `json:"severity"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FaultExecution struct {
	ID            string     `json:"id"`
	TemplateID    string     `json:"template_id"`
	DrillID       string     `json:"drill_id"`
	TargetType    string     `json:"target_type"`
	TargetID      string     `json:"target_id"`
	FaultType     string     `json:"fault_type"`
	Params        string     `json:"params"`
	Status        string     `json:"status"`
	Result        string     `json:"result"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	RollbackAt    *time.Time `json:"rollback_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type HADrill struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Plan        string     `json:"plan"`
	Status      string     `json:"status"`
	Result      string     `json:"result"`
	Score       int        `json:"score"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type HADrillReport struct {
	ID          string     `json:"id"`
	DrillID     string     `json:"drill_id"`
	Summary     string     `json:"summary"`
	Timeline    string     `json:"timeline"`
	Findings    string     `json:"findings"`
	Score       int        `json:"score"`
	GeneratedAt time.Time  `json:"generated_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type DiagnosisRecord struct {
	ID         string    `json:"id"`
	InstanceID string    `json:"instance_id"`
	Status     string    `json:"status"`
	Summary    string    `json:"summary"`
	Details    string    `json:"details"`
	Score      int       `json:"score"`
	CreatedAt  time.Time `json:"created_at"`
}

type SQLAdvice struct {
	ID         string    `json:"id"`
	SQLText    string    `json:"sql_text"`
	Explain    string    `json:"explain"`
	Advice     string    `json:"advice"`
	Score      int       `json:"score"`
	CreatedAt  time.Time `json:"created_at"`
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