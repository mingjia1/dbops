package models

import "time"

type KeyVersion struct {
	ID        string    `json:"id"`
	KeyDigest string    `json:"key_digest"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Note      string    `json:"note"`
}
