package models

import "time"

type ClusterDeployment struct {
	ID          string    `json:"id"`
	ClusterType string    `json:"cluster_type"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}