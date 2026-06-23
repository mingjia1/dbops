package models

import "time"

type ClusterDeployNode struct {
	ID           string     `json:"id"`
	DeploymentID string     `json:"deployment_id"`
	InstanceID   string     `json:"instance_id"`
	Host         string     `json:"host"`
	MySQLPort    int        `json:"mysql_port"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	CurrentStep  string     `json:"current_step"`
	Message      string     `json:"message"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ErrorMessage string     `json:"error_message"`
}
