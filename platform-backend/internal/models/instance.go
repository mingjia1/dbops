package models

import (
	"time"
)

type Instance struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name      string    `json:"name" gorm:"type:varchar(255);not null"`
	ClusterID string    `json:"cluster_id" gorm:"type:varchar(64);index"`
	HostID    *string   `json:"host_id" gorm:"type:varchar(64);index"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	
	Connection  InstanceConnection  `json:"connection" gorm:"foreignKey:InstanceID"`
	Version     InstanceVersion     `json:"version" gorm:"foreignKey:InstanceID"`
	Config      InstanceConfig      `json:"config" gorm:"foreignKey:InstanceID"`
	Status      InstanceStatus      `json:"status" gorm:"foreignKey:InstanceID"`
	Topology    InstanceTopology    `json:"topology" gorm:"foreignKey:InstanceID"`
}

type InstanceConnection struct {
	ID                string `gorm:"primaryKey;type:varchar(64)"`
	InstanceID        string `json:"instance_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	Host              string `json:"host" gorm:"type:varchar(255);not null"`
	Port              int    `json:"port" gorm:"type:int;not null"`
	Username          string `json:"username" gorm:"type:varchar(64);not null"`
	PasswordEncrypted string `json:"password_encrypted" gorm:"type:varchar(255);not null"`
	SSLEnabled        bool   `json:"ssl_enabled" gorm:"type:boolean;default:false"`
	// Install / upgrade paths. These let the platform install or upgrade to
	// ANY version from the catalog (not hard-coded to 5.7/8.0).
	Basedir    string `json:"basedir"     gorm:"type:varchar(255)"`
	Datadir    string `json:"datadir"     gorm:"type:varchar(255)"`
	OSUser     string `json:"os_user"     gorm:"type:varchar(64)"`
	PackageURL string `json:"package_url" gorm:"type:varchar(1024)"`
	VersionID  string `json:"version_id"  gorm:"type:varchar(64)"` // FK to version catalog id e.g. "mysql-8.0.36"
}

type InstanceVersion struct {
	ID            string    `gorm:"primaryKey;type:varchar(64)"`
	InstanceID    string    `json:"instance_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	Flavor        string    `json:"flavor" gorm:"type:varchar(32);not null"`
	Version       string    `json:"version" gorm:"type:varchar(32);not null"`
	FullVersion   string    `json:"full_version" gorm:"type:varchar(64);not null"`
	ReleaseDate   time.Time `json:"release_date" gorm:"type:date"`
	EOLDate       time.Time `json:"eol_date" gorm:"type:date"`
	IsLTS         bool      `json:"is_lts" gorm:"type:boolean;default:false"`
	Features      string    `json:"features" gorm:"type:text"`
	Engines       string    `json:"engines" gorm:"type:text"`
}

type InstanceConfig struct {
	ID                string `gorm:"primaryKey;type:varchar(64)"`
	InstanceID        string `json:"instance_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	ParameterTemplateID string `json:"parameter_template_id" gorm:"type:varchar(64)"`
	Parameters        string `json:"parameters" gorm:"type:text"`
	Charset           string `json:"charset" gorm:"type:varchar(32);default:'utf8mb4'"`
	Collation         string `json:"collation" gorm:"type:varchar(64);default:'utf8mb4_general_ci'"`
}

type InstanceStatus struct {
	ID                  string    `gorm:"primaryKey;type:varchar(64)"`
	InstanceID          string    `json:"instance_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	RunStatus           string    `json:"run_status" gorm:"type:varchar(32);default:'unknown'"`
	HealthStatus        string    `json:"health_status" gorm:"type:varchar(32);default:'unknown'"`
	Role                string    `json:"role" gorm:"type:varchar(32);default:'unknown'"`
	ReplicationStatus   string    `json:"replication_status" gorm:"type:varchar(32)"`
	SecondsBehindMaster int       `json:"seconds_behind_master" gorm:"type:int;default:-1"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

type InstanceTopology struct {
	ID              string `gorm:"primaryKey;type:varchar(64)"`
	InstanceID      string `json:"instance_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	ClusterID       string `json:"cluster_id" gorm:"type:varchar(64);index"`
	MasterID        string `json:"master_id" gorm:"type:varchar(64)"`
	SlaveIDs        string `json:"slave_ids" gorm:"type:text"`
	ReplicationMode string `json:"replication_mode" gorm:"type:varchar(32)"`
}