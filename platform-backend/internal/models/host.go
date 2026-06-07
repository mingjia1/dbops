package models

import "time"

type Host struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Address           string     `json:"address"`
	SSHPort           int        `json:"ssh_port"`
	SSHUser           string     `json:"ssh_user"`
	SSHAuthMethod     string     `json:"ssh_auth_method"`
	SSHCredential     string     `json:"-"`
	AgentPort         int        `json:"agent_port"`
	OSType            string     `json:"os_type"`
	Description       string     `json:"description"`
	Tags              string     `json:"tags"`
	Status            string     `json:"status"`
	LastCheckAt       *time.Time `json:"last_check_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
