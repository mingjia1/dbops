package models

import (
	"time"
)

type Instance struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ClusterID string    `json:"cluster_id"`
	HostID    *string   `json:"host_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Connection InstanceConnection `json:"connection"`
	Version    InstanceVersion    `json:"version"`
	Config     InstanceConfig     `json:"config"`
	Status     InstanceStatus     `json:"status"`
	Topology   InstanceTopology   `json:"topology"`
}

type InstanceConnection struct {
	ID                string `gorm:"primaryKey;type:varchar(64)"`
	InstanceID        string `json:"instance_id"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	PasswordEncrypted string `json:"password_encrypted"`
	SSLEnabled        bool   `json:"ssl_enabled"`
	// Install / upgrade paths. These let the platform install or upgrade to
	// ANY version from the catalog (not hard-coded to 5.7/8.0).
	Basedir    string `json:"basedir"`
	Datadir    string `json:"datadir"`
	OSUser     string `json:"os_user"`
	PackageURL string `json:"package_url"`
	VersionID  string `json:"version_id"` // FK to version catalog id e.g. "mysql-8.0.36"
}

type InstanceVersion struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)"`
	InstanceID  string    `json:"instance_id"`
	Flavor      string    `json:"flavor"`
	Version     string    `json:"version"`
	FullVersion string    `json:"full_version"`
	ReleaseDate time.Time `json:"release_date"`
	EOLDate     time.Time `json:"eol_date"`
	IsLTS       bool      `json:"is_lts"`
	Features    string    `json:"features"`
	Engines     string    `json:"engines"`
}

type InstanceConfig struct {
	ID                  string `gorm:"primaryKey;type:varchar(64)"`
	InstanceID          string `json:"instance_id"`
	ParameterTemplateID string `json:"parameter_template_id"`
	Parameters          string `json:"parameters"`
	Charset             string `json:"charset"`
	Collation           string `json:"collation"`
}

type InstanceStatus struct {
	ID                  string    `gorm:"primaryKey;type:varchar(64)"`
	InstanceID          string    `json:"instance_id"`
	RunStatus           string    `json:"run_status"`
	HealthStatus        string    `json:"health_status"`
	Role                string    `json:"role"`
	ReplicationStatus   string    `json:"replication_status"`
	SecondsBehindMaster int       `json:"seconds_behind_master"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type InstanceTopology struct {
	ID              string `gorm:"primaryKey;type:varchar(64)"`
	InstanceID      string `json:"instance_id"`
	ClusterID       string `json:"cluster_id"`
	MasterID        string `json:"master_id"`
	SlaveIDs        string `json:"slave_ids"`
	ReplicationMode string `json:"replication_mode"`
}
