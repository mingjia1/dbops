package models

import "time"

type ClusterDeployment struct {
	ID           string     `json:"id"`
	ClusterType  string     `json:"cluster_type"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	RequestJSON  string     `json:"request_json"`
	PlanJSON     string     `json:"plan_json"`
	CustomJSON   string     `json:"custom_json"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ErrorMessage string     `json:"error_message"`
	ClusterID    string     `json:"cluster_id"`
	DisplayName  string     `json:"display_name"`
	Arch         string     `json:"arch"`
	Nodes        int        `json:"nodes"`
	MySQLVersion string     `json:"mysql_version"`
	ConfigJSON   string     `json:"config_json"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}