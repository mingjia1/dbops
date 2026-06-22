package models

import "time"

type ClusterCredential struct {
	ID          string     `json:"id"`
	ClusterID   string     `json:"cluster_id"`
	AccountType string     `json:"account_type"`
	Username    string     `json:"username"`
	PasswordEnc string     `json:"password_encrypted"`
	CreatedAt   time.Time  `json:"created_at"`
	RotatedAt   *time.Time `json:"rotated_at"`
}
