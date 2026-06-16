package models

import "time"

type MaskingAlgorithm string

const (
	MaskingMD5   MaskingAlgorithm = "md5"
	MaskingMask  MaskingAlgorithm = "mask"
	MaskingReplace MaskingAlgorithm = "replace"
)

type MaskingRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	FieldPath   string            `json:"field_path"`
	Pattern     string            `json:"pattern"`
	Algorithm   MaskingAlgorithm  `json:"algorithm"`
	Replacement string            `json:"replacement"`
	Roles       []string          `json:"roles"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
