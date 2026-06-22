package services

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type ConfigRenderer struct{}

func NewConfigRenderer() *ConfigRenderer {
	return &ConfigRenderer{}
}

type MySQLConfig struct {
	Flavor            string
	Version           string
	MajorMinor        string
	Port              int
	ServerID          int
	Datadir           string
	Basedir           string
	GTIDMode          bool
	GTIDDomainID      int
	EnforceGTID       bool
	AuthPlugin        string
	BinaryName        string
	CharacterSet      string
	MaxConnections    int
	InnodbBufferPool  string
	LogBin            bool
	BinlogFormat      string
	RelayLog          string
	ReportHost        string
	ReportPort        int
	GroupReplication  bool
	GroupLocalAddr    string
	GroupSeeds        string
}

func NewMySQLConfig(flavor, version string, port, serverID int) *MySQLConfig {
	cfg := &MySQLConfig{
		Flavor:           flavor,
		Version:          version,
		MajorMinor:       majorMinor(version),
		Port:             port,
		ServerID:         serverID,
		CharacterSet:     "utf8mb4",
		MaxConnections:   500,
		InnodbBufferPool: "1G",
		BinlogFormat:     "ROW",
		LogBin:           true,
		ReportPort:       port,
	}

	switch flavor {
	case "mariadb":
		cfg.GTIDDomainID = serverID
		cfg.BinaryName = "mariadbd"
		cfg.GTIDMode = false
		cfg.EnforceGTID = false
		cfg.AuthPlugin = ""
	case "percona":
		cfg.GTIDMode = true
		cfg.EnforceGTID = true
		cfg.AuthPlugin = "mysql_native_password"
		cfg.BinaryName = "mysqld"
	default:
		cfg.GTIDMode = true
		cfg.EnforceGTID = true
		cfg.BinaryName = "mysqld"
		if cfg.MajorMinor == "5.7" {
			cfg.AuthPlugin = ""
		} else {
			cfg.AuthPlugin = "mysql_native_password"
		}
	}
	return cfg
}

func majorMinor(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}

func (r *ConfigRenderer) Render(cfg *MySQLConfig) (string, error) {
	tmpl := r.selectTemplate(cfg)
	t, err := template.New("mycnf").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func (r *ConfigRenderer) selectTemplate(cfg *MySQLConfig) string {
	switch cfg.Flavor {
	case "mariadb":
		return mariadbTemplate
	case "percona":
		return perconaTemplate
	default:
		if cfg.MajorMinor == "5.7" {
			return mysql57Template
		}
		return mysql80Template
	}
}

var mysql57Template = `[mysqld]
port={{.Port}}
server-id={{.ServerID}}
datadir={{.Datadir}}
basedir={{.Basedir}}
socket=/tmp/mysql_{{.Port}}.sock
pid-file=/tmp/mysql_{{.Port}}.pid

gtid_mode=ON
enforce_gtid_consistency=ON

{{- if .LogBin }}
log_bin={{.Datadir}}/binlog
binlog_format={{.BinlogFormat}}
{{- end }}
relay_log={{.Datadir}}/relay-log
report_host={{.ReportHost}}
report_port={{.ReportPort}}

character-set-server={{.CharacterSet}}
max_connections={{.MaxConnections}}
innodb_buffer_pool_size={{.InnodbBufferPool}}
`

var mysql80Template = `[mysqld]
port={{.Port}}
server-id={{.ServerID}}
datadir={{.Datadir}}
basedir={{.Basedir}}
socket=/tmp/mysql_{{.Port}}.sock
pid-file=/tmp/mysql_{{.Port}}.pid

gtid_mode=ON
enforce_gtid_consistency=ON

{{- if .LogBin }}
log_bin={{.Datadir}}/binlog
binlog_format={{.BinlogFormat}}
{{- end }}
relay_log={{.Datadir}}/relay-log
report_host={{.ReportHost}}
report_port={{.ReportPort}}

character-set-server={{.CharacterSet}}
max_connections={{.MaxConnections}}
innodb_buffer_pool_size={{.InnodbBufferPool}}
{{- if .AuthPlugin }}
default_authentication_plugin={{.AuthPlugin}}
{{- end }}
{{- if .GroupReplication }}
plugin_load_add='group_replication.so'
group_replication_group_name={{.GroupSeeds}}
group_replication_local_address={{.GroupLocalAddr}}
group_replication_group_seeds={{.GroupSeeds}}
group_replication_start_on_boot=OFF
{{- end }}
`

var mariadbTemplate = `[mysqld]
port={{.Port}}
server-id={{.ServerID}}
datadir={{.Datadir}}
basedir={{.Basedir}}
socket=/tmp/mysql_{{.Port}}.sock
pid-file=/tmp/mysql_{{.Port}}.pid

gtid_domain_id={{.GTIDDomainID}}

{{- if .LogBin }}
log_bin={{.Datadir}}/binlog
binlog_format={{.BinlogFormat}}
{{- end }}
relay_log={{.Datadir}}/relay-log
report_host={{.ReportHost}}
report_port={{.ReportPort}}

character-set-server={{.CharacterSet}}
max_connections={{.MaxConnections}}
innodb_buffer_pool_size={{.InnodbBufferPool}}
`

var perconaTemplate = `[mysqld]
port={{.Port}}
server-id={{.ServerID}}
datadir={{.Datadir}}
basedir={{.Basedir}}
socket=/tmp/mysql_{{.Port}}.sock
pid-file=/tmp/mysql_{{.Port}}.pid

gtid_mode=ON
enforce_gtid_consistency=ON

{{- if .LogBin }}
log_bin={{.Datadir}}/binlog
binlog_format={{.BinlogFormat}}
{{- end }}
relay_log={{.Datadir}}/relay-log
report_host={{.ReportHost}}
report_port={{.ReportPort}}

character-set-server={{.CharacterSet}}
max_connections={{.MaxConnections}}
innodb_buffer_pool_size={{.InnodbBufferPool}}
{{- if .AuthPlugin }}
default_authentication_plugin={{.AuthPlugin}}
{{- end }}
`
