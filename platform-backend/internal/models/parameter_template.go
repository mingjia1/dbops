package models

import (
	"time"
)

type ParameterTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	IsPreset    bool      `json:"is_preset"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Versions   []ParameterTemplateVersion   `json:"versions"`
	Parameters []ParameterTemplateParameter `json:"parameters"`
}

type ParameterTemplateVersion struct {
	ID          string    `json:"id"`
	TemplateID  string    `json:"template_id"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`

	Template ParameterTemplate `json:"template"`
}

type ParameterTemplateParameter struct {
	ID            string `json:"id"`
	TemplateID    string `json:"template_id"`
	VersionID     string `json:"version_id"`
	ParameterName string `json:"parameter_name"`
	Value         string `json:"value"`
	DataType      string `json:"data_type"`
	MinValue      string `json:"min_value"`
	MaxValue      string `json:"max_value"`
	Unit          string `json:"unit"`
	Description   string `json:"description"`
	IsDynamic     bool   `json:"is_dynamic"`
	IsMandatory   bool   `json:"is_mandatory"`
	Category      string `json:"category"`

	Template ParameterTemplate       `json:"template"`
	Version  ParameterTemplateVersion `json:"version"`
}

type PresetTemplateCategory string

const (
	PresetCategorySmall      PresetTemplateCategory = "small"
	PresetCategoryMedium     PresetTemplateCategory = "medium"
	PresetCategoryLarge      PresetTemplateCategory = "large"
	PresetCategoryMemoryOpt  PresetTemplateCategory = "memory_optimized"
	PresetCategoryOLTP       PresetTemplateCategory = "oltp"
	PresetCategoryOLAP       PresetTemplateCategory = "olap"
)

type ParameterDataType string

const (
	DataTypeInt     ParameterDataType = "int"
	DataTypeBool    ParameterDataType = "bool"
	DataTypeString  ParameterDataType = "string"
	DataTypeSize    ParameterDataType = "size"
	DataTypePercent ParameterDataType = "percent"
	DataTypeTime    ParameterDataType = "time"
)