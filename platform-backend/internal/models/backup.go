package models

import (
	"time"
)

type BackupPolicy struct {
	ID           string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	InstanceID   string    `json:"instance_id" gorm:"type:varchar(64);not null;index"`
	BackupType   string    `json:"backup_type" gorm:"type:varchar(32);not null"`
	Schedule     string    `json:"schedule" gorm:"type:varchar(128);not null"`
	RetentionDays int      `json:"retention_days" gorm:"type:int;default:7"`
	StorageType  string    `json:"storage_type" gorm:"type:varchar(32);default:'local'"`
	StoragePath  string    `json:"storage_path" gorm:"type:varchar(512)"`
	Enabled      bool      `json:"enabled" gorm:"type:boolean;default:true"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	
	Records []BackupRecord `json:"records" gorm:"foreignKey:PolicyID"`
}

type BackupRecord struct {
	ID           string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	PolicyID     string    `json:"policy_id" gorm:"type:varchar(64);index"`
	InstanceID   string    `json:"instance_id" gorm:"type:varchar(64);index"`
	BackupType   string    `json:"backup_type" gorm:"type:varchar(32);not null"`
	StartedAt    time.Time `json:"started_at" gorm:"type:timestamp"`
	CompletedAt  time.Time `json:"completed_at" gorm:"type:timestamp"`
	Status       string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	FilePath     string    `json:"file_path" gorm:"type:varchar(512)"`
	FileSize     int64     `json:"file_size" gorm:"type:bigint"`
	Checksum     string    `json:"checksum" gorm:"type:varchar(128)"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	
	Restores []RestoreRecord `json:"restores" gorm:"foreignKey:BackupID"`
}

type RestoreRecord struct {
	ID              string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	BackupID        string    `json:"backup_id" gorm:"type:varchar(64);index"`
	TargetInstanceID string   `json:"target_instance_id" gorm:"type:varchar(64);index"`
	StartedAt       time.Time `json:"started_at" gorm:"type:timestamp"`
	CompletedAt     time.Time `json:"completed_at" gorm:"type:timestamp"`
	Status          string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	RestorePoint    time.Time `json:"restore_point" gorm:"type:timestamp"`
	RestoredTables  string    `json:"restored_tables" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
}