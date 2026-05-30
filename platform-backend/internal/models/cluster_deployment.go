package models

import "time"

type ClusterDeployment struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	ClusterType string    `json:"cluster_type" gorm:"type:varchar(32);not null"`
	Name        string    `json:"name" gorm:"type:varchar(255);not null"`
	Status      string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}