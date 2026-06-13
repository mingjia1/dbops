package models

import (
	"time"
)

type Task struct {
	ID           string    `json:"id"`
	TaskType     string    `json:"task_type"`
	InstanceID   string    `json:"instance_id"`
	Status       string    `json:"status"`
	Progress     int       `json:"progress"`
	CreatedAt    time.Time `json:"created_at"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	ErrorMessage string    `json:"error_message"`
	
	Workflow    TaskWorkflow     `json:"workflow"`
	Checkpoints []TaskCheckpoint `json:"checkpoints"`
	Logs        []TaskLog        `json:"logs"`
}

type TaskWorkflow struct {
	ID                string    `gorm:"primaryKey;type:varchar(64)"`
	TaskID            string    `json:"task_id"`
	WorkflowID        string    `json:"workflow_id"`
	WorkflowDefinition string   `json:"workflow_definition"`
	CurrentStage      string    `json:"current_stage"`
	CompletedStages   string    `json:"completed_stages"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type TaskCheckpoint struct {
	ID            string    `gorm:"primaryKey;type:varchar(64)"`
	TaskID        string    `json:"task_id"`
	CheckpointID  string    `json:"checkpoint_id"`
	Stage         string    `json:"stage"`
	CheckpointData string   `json:"checkpoint_data"`
	CreatedAt     time.Time `json:"created_at"`
}

type TaskLog struct {
	ID        string    `gorm:"primaryKey;type:varchar(64)"`
	TaskID    string    `json:"task_id"`
	LogID     string    `json:"log_id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Context   string    `json:"context"`
}