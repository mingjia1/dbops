package repositories

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

var InitialSchema = []string{
	`CREATE TABLE IF NOT EXISTS instances (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		cluster_id VARCHAR(64),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS instance_connections (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		host VARCHAR(255) NOT NULL,
		port INT NOT NULL,
		username VARCHAR(64) NOT NULL,
		password_encrypted VARCHAR(255) NOT NULL,
		ssl_enabled BOOLEAN DEFAULT FALSE
	)`,
	
	`CREATE TABLE IF NOT EXISTS instance_versions (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		flavor VARCHAR(32) NOT NULL,
		version VARCHAR(32) NOT NULL,
		full_version VARCHAR(64) NOT NULL,
		release_date DATE,
		eol_date DATE,
		is_lts BOOLEAN DEFAULT FALSE,
		features TEXT,
		engines TEXT
	)`,
	
	`CREATE TABLE IF NOT EXISTS instance_configs (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		parameter_template_id VARCHAR(64),
		parameters TEXT,
		charset VARCHAR(32) DEFAULT 'utf8mb4',
		collation VARCHAR(64) DEFAULT 'utf8mb4_general_ci'
	)`,
	
	`CREATE TABLE IF NOT EXISTS instance_statuses (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		run_status VARCHAR(32) DEFAULT 'unknown',
		health_status VARCHAR(32) DEFAULT 'unknown',
		role VARCHAR(32) DEFAULT 'unknown',
		replication_status VARCHAR(32),
		seconds_behind_master INT DEFAULT -1,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS instance_topologies (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		cluster_id VARCHAR(64),
		master_id VARCHAR(64),
		slave_ids TEXT,
		replication_mode VARCHAR(32)
	)`,
	
	`CREATE TABLE IF NOT EXISTS tasks (
		id VARCHAR(64) PRIMARY KEY,
		task_type VARCHAR(32) NOT NULL,
		instance_id VARCHAR(64) REFERENCES instances(id) ON DELETE CASCADE,
		status VARCHAR(32) DEFAULT 'pending',
		progress INT DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		error_message TEXT
	)`,
	
	`CREATE TABLE IF NOT EXISTS task_workflows (
		id VARCHAR(64) PRIMARY KEY,
		task_id VARCHAR(64) UNIQUE NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		workflow_id VARCHAR(64),
		workflow_definition TEXT,
		current_stage VARCHAR(64),
		completed_stages TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS task_checkpoints (
		id VARCHAR(64) PRIMARY KEY,
		task_id VARCHAR(64) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		checkpoint_id VARCHAR(64),
		stage VARCHAR(64),
		checkpoint_data TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS task_logs (
		id VARCHAR(64) PRIMARY KEY,
		task_id VARCHAR(64) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		log_id VARCHAR(64),
		timestamp TIMESTAMP NOT NULL,
		level VARCHAR(16) NOT NULL,
		message TEXT NOT NULL,
		context TEXT
	)`,
	
	`CREATE TABLE IF NOT EXISTS backup_policies (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		backup_type VARCHAR(32) NOT NULL,
		schedule VARCHAR(128) NOT NULL,
		retention_days INT DEFAULT 7,
		storage_type VARCHAR(32) DEFAULT 'local',
		storage_path VARCHAR(512),
		enabled BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS backup_records (
		id VARCHAR(64) PRIMARY KEY,
		policy_id VARCHAR(64) REFERENCES backup_policies(id) ON DELETE CASCADE,
		instance_id VARCHAR(64) NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		backup_type VARCHAR(32) NOT NULL,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		status VARCHAR(32) DEFAULT 'pending',
		file_path VARCHAR(512),
		file_size BIGINT,
		checksum VARCHAR(128),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS restore_records (
		id VARCHAR(64) PRIMARY KEY,
		backup_id VARCHAR(64) REFERENCES backup_records(id) ON DELETE CASCADE,
		target_instance_id VARCHAR(64) REFERENCES instances(id) ON DELETE CASCADE,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		status VARCHAR(32) DEFAULT 'pending',
		restore_point TIMESTAMP,
		restored_tables TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(64) PRIMARY KEY,
		username VARCHAR(64) UNIQUE NOT NULL,
		password VARCHAR(255) NOT NULL,
		email VARCHAR(128) UNIQUE,
		role VARCHAR(32) NOT NULL DEFAULT 'operator',
		status VARCHAR(16) DEFAULT 'active',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS alert_rules (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(128) NOT NULL,
		metric VARCHAR(128) NOT NULL,
		condition VARCHAR(32) NOT NULL,
		threshold DECIMAL(10,2) NOT NULL,
		duration_seconds INT DEFAULT 60,
		severity VARCHAR(16) NOT NULL,
		notification_channels TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS alert_records (
		id VARCHAR(64) PRIMARY KEY,
		rule_id VARCHAR(64) REFERENCES alert_rules(id) ON DELETE CASCADE,
		instance_id VARCHAR(64) REFERENCES instances(id) ON DELETE CASCADE,
		triggered_at TIMESTAMP NOT NULL,
		resolved_at TIMESTAMP,
		status VARCHAR(32) DEFAULT 'firing',
		severity VARCHAR(16) NOT NULL,
		value DECIMAL(10,2),
		message TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS parameter_templates (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) UNIQUE NOT NULL,
		description TEXT,
		category VARCHAR(64) NOT NULL,
		is_preset BOOLEAN DEFAULT FALSE,
		created_by VARCHAR(64),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS parameter_template_versions (
		id VARCHAR(64) PRIMARY KEY,
		template_id VARCHAR(64) NOT NULL REFERENCES parameter_templates(id) ON DELETE CASCADE,
		version VARCHAR(32) NOT NULL,
		description TEXT,
		is_active BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	
	`CREATE TABLE IF NOT EXISTS parameter_template_parameters (
		id VARCHAR(64) PRIMARY KEY,
		template_id VARCHAR(64) NOT NULL REFERENCES parameter_templates(id) ON DELETE CASCADE,
		version_id VARCHAR(64) REFERENCES parameter_template_versions(id) ON DELETE CASCADE,
		parameter_name VARCHAR(128) NOT NULL,
		value TEXT NOT NULL,
		data_type VARCHAR(32) NOT NULL,
		min_value TEXT,
		max_value TEXT,
		unit VARCHAR(32),
		description TEXT,
		is_dynamic BOOLEAN DEFAULT TRUE,
		is_mandatory BOOLEAN DEFAULT FALSE,
		category VARCHAR(64)
	)`,
	
	`CREATE INDEX IF NOT EXISTS idx_instances_cluster ON instances(cluster_id)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(task_type)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_instance ON tasks(instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_backup_policies_instance ON backup_policies(instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_backup_records_instance ON backup_records(instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_alert_records_instance ON alert_records(instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_templates_category ON parameter_templates(category)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_templates_is_preset ON parameter_templates(is_preset)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_versions_template ON parameter_template_versions(template_id)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_parameters_template ON parameter_template_parameters(template_id)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_parameters_version ON parameter_template_parameters(version_id)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_parameters_name ON parameter_template_parameters(parameter_name)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_parameters_category ON parameter_template_parameters(category)`,
}

func RunMigrations(ctx context.Context, conn *pgx.Conn) error {
	for _, sql := range InitialSchema {
		_, err := conn.Exec(ctx, sql)
		if err != nil {
			return fmt.Errorf("failed to execute migration: %w\nSQL: %s", err, sql)
		}
	}
	return nil
}