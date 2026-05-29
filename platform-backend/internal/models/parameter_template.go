package models

import (
	"time"
)

type ParameterTemplate struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name        string    `json:"name" gorm:"type:varchar(255);not null;uniqueIndex"`
	Description string    `json:"description" gorm:"type:text"`
	Category    string    `json:"category" gorm:"type:varchar(64);not null;index"`
	IsPreset    bool      `json:"is_preset" gorm:"type:boolean;default:false;index"`
	CreatedBy   string    `json:"created_by" gorm:"type:varchar(64)"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	Versions   []ParameterTemplateVersion   `json:"versions" gorm:"foreignKey:TemplateID"`
	Parameters []ParameterTemplateParameter `json:"parameters" gorm:"foreignKey:TemplateID"`
}

type ParameterTemplateVersion struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TemplateID  string    `json:"template_id" gorm:"type:varchar(64);not null;index"`
	Version     string    `json:"version" gorm:"type:varchar(32);not null"`
	Description string    `json:"description" gorm:"type:text"`
	IsActive    bool      `json:"is_active" gorm:"type:boolean;default:true;index"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`

	Template ParameterTemplate `json:"template" gorm:"foreignKey:TemplateID"`
}

type ParameterTemplateParameter struct {
	ID            string `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TemplateID    string `json:"template_id" gorm:"type:varchar(64);not null;index"`
	VersionID     string `json:"version_id" gorm:"type:varchar(64);index"`
	ParameterName string `json:"parameter_name" gorm:"type:varchar(128);not null;index"`
	Value         string `json:"value" gorm:"type:text;not null"`
	DataType      string `json:"data_type" gorm:"type:varchar(32);not null"`
	MinValue      string `json:"min_value" gorm:"type:text"`
	MaxValue      string `json:"max_value" gorm:"type:text"`
	Unit          string `json:"unit" gorm:"type:varchar(32)"`
	Description   string `json:"description" gorm:"type:text"`
	IsDynamic     bool   `json:"is_dynamic" gorm:"type:boolean;default:true"`
	IsMandatory   bool   `json:"is_mandatory" gorm:"type:boolean;default:false"`
	Category      string `json:"category" gorm:"type:varchar(64);index"`

	Template ParameterTemplate       `json:"template" gorm:"foreignKey:TemplateID"`
	Version  ParameterTemplateVersion `json:"version" gorm:"foreignKey:VersionID"`
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