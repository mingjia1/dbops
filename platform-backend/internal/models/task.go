package models

import (
	"time"
)

type Task struct {
	ID           string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TaskType     string    `json:"task_type" gorm:"type:varchar(32);not null;index"`
	InstanceID   string    `json:"instance_id" gorm:"type:varchar(64);index"`
	Status       string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	Progress     int       `json:"progress" gorm:"type:int;default:0"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	StartedAt    time.Time `json:"started_at" gorm:"type:timestamp"`
	CompletedAt  time.Time `json:"completed_at" gorm:"type:timestamp"`
	ErrorMessage string    `json:"error_message" gorm:"type:text"`
	
	Workflow    TaskWorkflow     `json:"workflow" gorm:"foreignKey:TaskID"`
	Checkpoints []TaskCheckpoint `json:"checkpoints" gorm:"foreignKey:TaskID"`
	Logs        []TaskLog        `json:"logs" gorm:"foreignKey:TaskID"`
}

type TaskWorkflow struct {
	ID                string    `gorm:"primaryKey;type:varchar(64)"`
	TaskID            string    `json:"task_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	WorkflowID        string    `json:"workflow_id" gorm:"type:varchar(64)"`
	WorkflowDefinition string   `json:"workflow_definition" gorm:"type:text"`
	CurrentStage      string    `json:"current_stage" gorm:"type:varchar(64)"`
	CompletedStages   string    `json:"completed_stages" gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

type TaskCheckpoint struct {
	ID            string    `gorm:"primaryKey;type:varchar(64)"`
	TaskID        string    `json:"task_id" gorm:"type:varchar(64);index;not null"`
	CheckpointID  string    `json:"checkpoint_id" gorm:"type:varchar(64)"`
	Stage         string    `json:"stage" gorm:"type:varchar(64)"`
	CheckpointData string   `json:"checkpoint_data" gorm:"type:text"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type TaskLog struct {
	ID        string    `gorm:"primaryKey;type:varchar(64)"`
	TaskID    string    `json:"task_id" gorm:"type:varchar(64);index;not null"`
	LogID     string    `json:"log_id" gorm:"type:varchar(64)"`
	Timestamp time.Time `json:"timestamp" gorm:"type:timestamp;not null"`
	Level     string    `json:"level" gorm:"type:varchar(16);not null"`
	Message   string    `json:"message" gorm:"type:text;not null"`
	Context   string    `json:"context" gorm:"type:text"`
}