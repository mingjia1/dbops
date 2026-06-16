package models

import "time"

type License struct {
	ID         string    `json:"id"`
	Tier       string    `json:"tier"`
	LicenseKey string    `json:"license_key"`
	IssuedTo   string    `json:"issued_to"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	MaxNodes   int       `json:"max_nodes"`
	LicenseJSON string   `json:"license_json"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}
