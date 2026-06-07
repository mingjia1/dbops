package models

import (
	"time"
)

type BackupPolicy struct {
	ID           string    `json:"id"`
	InstanceID   string    `json:"instance_id"`
	BackupType   string    `json:"backup_type"`
	Schedule     string    `json:"schedule"`
	RetentionDays int      `json:"retention_days"`
	StorageType  string    `json:"storage_type"`
	StoragePath  string    `json:"storage_path"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	
	Records []BackupRecord `json:"records"`
}

type BackupRecord struct {
	ID           string    `json:"id"`
	PolicyID     string    `json:"policy_id"`
	InstanceID   string    `json:"instance_id"`
	BackupType   string    `json:"backup_type"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Status       string    `json:"status"`
	FilePath     string    `json:"file_path"`
	FileSize     int64     `json:"file_size"`
	Checksum     string    `json:"checksum"`
	CreatedAt    time.Time `json:"created_at"`
	
	Restores []RestoreRecord `json:"restores"`
}

type RestoreRecord struct {
	ID              string    `json:"id"`
	BackupID        string    `json:"backup_id"`
	TargetInstanceID string   `json:"target_instance_id"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	Status          string    `json:"status"`
	RestorePoint    time.Time `json:"restore_point"`
	RestoredTables  string    `json:"restored_tables"`
	CreatedAt       time.Time `json:"created_at"`
}