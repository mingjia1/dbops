package services

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type ConfigRenderer struct {
	// TemplateDir 是 .tmpl 文件的目录路径。空字符串表示使用内联模板。
	TemplateDir string
}

func NewConfigRenderer() *ConfigRenderer {
	return &ConfigRenderer{}
}

// SetTemplateDir 设置外部模板文件目录。
// 如果目录存在，Render() 将优先从该目录加载 .tmpl 文件；否则使用内联模板。
func (r *ConfigRenderer) SetTemplateDir(dir string) {
	r.TemplateDir = dir
}

// templateNameFor 返回对应模板的文件名
func templateNameFor(cfg *MySQLConfig) string {
	switch cfg.Flavor {
	case "mariadb":
		return "mariadb10.cnf.tmpl"
	case "percona":
		return "percona80.cnf.tmpl"
	default:
		if cfg.MajorMinor == "5.7" {
			return "mysql57.cnf.tmpl"
		}
		return "mysql80.cnf.tmpl"
	}
}

func (r *ConfigRenderer) Render(cfg *MySQLConfig) (string, error) {
	tmplContent := r.loadTemplate(cfg)
	t, err := template.New("mycnf").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// loadTemplate 优先从外部 .tmpl 文件加载，失败时回退到内联模板
func (r *ConfigRenderer) loadTemplate(cfg *MySQLConfig) string {
	// 尝试从外部文件加载
	if r.TemplateDir != "" {
		name := templateNameFor(cfg)
		path := filepath.Join(r.TemplateDir, name)
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return string(data)
		}
	}
	// 回退：尝试从默认的相对路径加载（相对于工作目录）
	name := templateNameFor(cfg)
	defaultPath := filepath.Join("internal", "templates", name)
	if data, err := os.ReadFile(defaultPath); err == nil && len(data) > 0 {
		return string(data)
	}

	// 最终回退：使用内联模板
	return inlineTemplate(cfg)
}

// inlineTemplate 返回编译时内置的模板内容（外部文件不存在时的兜底）
func inlineTemplate(cfg *MySQLConfig) string {
	switch cfg.Flavor {
	case "mariadb":
		return mariadbInline
	case "percona":
		return perconaInline
	default:
		if cfg.MajorMinor == "5.7" {
			return mysql57Inline
		}
		return mysql80Inline
	}
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
	GroupName         string // UUID required by MySQL
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
		// 8.0 默认 caching_sha2；需要 native 时由部署方显式设置 AuthPlugin。
		cfg.AuthPlugin = ""
		cfg.BinaryName = "mysqld"
	default:
		cfg.GTIDMode = true
		cfg.EnforceGTID = true
		cfg.BinaryName = "mysqld"
		// 不强制 default_authentication_plugin，沿用服务端默认。
		cfg.AuthPlugin = ""
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

var mysql57Inline = `[mysqld]
port={{.Port}}
server-id={{.ServerID}}
datadir={{.Datadir}}
basedir={{.Basedir}}
socket={{.Datadir}}/mysql.sock
pid-file={{.Datadir}}/mysql.pid
log-error={{.Datadir}}/error.log

gtid_mode=ON
enforce_gtid_consistency=ON

{{- if .LogBin }}
log_bin={{.Datadir}}/binlog
binlog_format={{.BinlogFormat}}
expire_logs_days=7
sync_binlog=1
{{- end }}
relay_log={{.Datadir}}/relay-log
relay_log_recovery=ON
report_host={{.ReportHost}}
report_port={{.ReportPort}}

character-set-server={{.CharacterSet}}
max_connections={{.MaxConnections}}
innodb_buffer_pool_size={{.InnodbBufferPool}}
innodb_flush_log_at_trx_commit=1
skip_name_resolve=ON
`

var mysql80Inline = `[mysqld]
port={{.Port}}
server-id={{.ServerID}}
datadir={{.Datadir}}
basedir={{.Basedir}}
socket={{.Datadir}}/mysql.sock
pid-file={{.Datadir}}/mysql.pid
log-error={{.Datadir}}/error.log

gtid_mode=ON
enforce_gtid_consistency=ON

{{- if .LogBin }}
log_bin={{.Datadir}}/binlog
binlog_format={{.BinlogFormat}}
binlog_expire_logs_seconds=604800
sync_binlog=1
{{- end }}
relay_log={{.Datadir}}/relay-log
relay_log_recovery=ON
report_host={{.ReportHost}}
report_port={{.ReportPort}}

character-set-server={{.CharacterSet}}
max_connections={{.MaxConnections}}
innodb_buffer_pool_size={{.InnodbBufferPool}}
innodb_flush_log_at_trx_commit=1
skip_name_resolve=ON
{{- if .AuthPlugin }}
default_authentication_plugin={{.AuthPlugin}}
{{- end }}
{{- if .GroupReplication }}
plugin_load_add='group_replication.so'
group_replication_group_name={{.GroupName}}
group_replication_local_address={{.GroupLocalAddr}}
group_replication_group_seeds={{.GroupSeeds}}
group_replication_start_on_boot=OFF
{{- end }}
`

var mariadbInline = `[mariadbd]
port={{.Port}}
server-id={{.ServerID}}
datadir={{.Datadir}}
basedir={{.Basedir}}
socket={{.Datadir}}/mysql.sock
pid-file={{.Datadir}}/mysql.pid
log-error={{.Datadir}}/error.log

gtid_domain_id={{.GTIDDomainID}}

{{- if .LogBin }}
log_bin={{.Datadir}}/binlog
binlog_format={{.BinlogFormat}}
expire_logs_days=7
sync_binlog=1
{{- end }}
relay_log={{.Datadir}}/relay-log
relay_log_recovery=ON
report_host={{.ReportHost}}
report_port={{.ReportPort}}

character-set-server={{.CharacterSet}}
max_connections={{.MaxConnections}}
innodb_buffer_pool_size={{.InnodbBufferPool}}
innodb_flush_log_at_trx_commit=1
skip_name_resolve=ON
`

var perconaInline = `[mysqld]
port={{.Port}}
server-id={{.ServerID}}
datadir={{.Datadir}}
basedir={{.Basedir}}
socket={{.Datadir}}/mysql.sock
pid-file={{.Datadir}}/mysql.pid
log-error={{.Datadir}}/error.log

gtid_mode=ON
enforce_gtid_consistency=ON

{{- if .LogBin }}
log_bin={{.Datadir}}/binlog
binlog_format={{.BinlogFormat}}
binlog_expire_logs_seconds=604800
sync_binlog=1
{{- end }}
relay_log={{.Datadir}}/relay-log
relay_log_recovery=ON
report_host={{.ReportHost}}
report_port={{.ReportPort}}

character-set-server={{.CharacterSet}}
max_connections={{.MaxConnections}}
innodb_buffer_pool_size={{.InnodbBufferPool}}
innodb_flush_log_at_trx_commit=1
skip_name_resolve=ON
{{- if .AuthPlugin }}
default_authentication_plugin={{.AuthPlugin}}
{{- end }}
`
