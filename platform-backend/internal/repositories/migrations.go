package repositories

import (
	"context"
	"database/sql"
	"fmt"
)

var InitialSchema = []string{
	`CREATE TABLE IF NOT EXISTS instances (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		cluster_id VARCHAR(64),
		host_id VARCHAR(64),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_instances_cluster (cluster_id),
		INDEX idx_instances_host (host_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS instance_connections (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL,
		host VARCHAR(255) NOT NULL,
		port INT NOT NULL,
		username VARCHAR(64) NOT NULL,
		password_encrypted VARCHAR(255) NOT NULL,
		ssl_enabled TINYINT(1) DEFAULT 0,
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS instance_versions (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL,
		flavor VARCHAR(32) NOT NULL,
		version VARCHAR(32) NOT NULL,
		full_version VARCHAR(64) NOT NULL,
		release_date DATE,
		eol_date DATE,
		is_lts TINYINT(1) DEFAULT 0,
		features TEXT,
		engines TEXT,
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS instance_configs (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL,
		parameter_template_id VARCHAR(64),
		parameters TEXT,
		charset VARCHAR(32) DEFAULT 'utf8mb4',
		collation VARCHAR(64) DEFAULT 'utf8mb4_general_ci',
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS instance_statuses (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL,
		run_status VARCHAR(32) DEFAULT 'unknown',
		health_status VARCHAR(32) DEFAULT 'unknown',
		role VARCHAR(32) DEFAULT 'unknown',
		replication_status VARCHAR(32),
		seconds_behind_master INT DEFAULT -1,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS instance_topologies (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) UNIQUE NOT NULL,
		cluster_id VARCHAR(64),
		master_id VARCHAR(64),
		slave_ids TEXT,
		replication_mode VARCHAR(32),
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS tasks (
		id VARCHAR(64) PRIMARY KEY,
		task_type VARCHAR(32) NOT NULL,
		instance_id VARCHAR(64),
		status VARCHAR(32) DEFAULT 'pending',
		progress INT DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL,
		error_message TEXT,
		INDEX idx_tasks_type (task_type),
		INDEX idx_tasks_instance (instance_id),
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS task_workflows (
		id VARCHAR(64) PRIMARY KEY,
		task_id VARCHAR(64) UNIQUE NOT NULL,
		workflow_id VARCHAR(64),
		workflow_definition TEXT,
		current_stage VARCHAR(64),
		completed_stages TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS task_checkpoints (
		id VARCHAR(64) PRIMARY KEY,
		task_id VARCHAR(64) NOT NULL,
		checkpoint_id VARCHAR(64),
		stage VARCHAR(64),
		checkpoint_data TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS task_logs (
		id VARCHAR(64) PRIMARY KEY,
		task_id VARCHAR(64) NOT NULL,
		log_id VARCHAR(64),
		timestamp TIMESTAMP NOT NULL,
		level VARCHAR(16) NOT NULL,
		message TEXT NOT NULL,
		context TEXT,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS backup_policies (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) NOT NULL,
		backup_type VARCHAR(32) NOT NULL,
		schedule VARCHAR(128) NOT NULL,
		retention_days INT DEFAULT 7,
		storage_type VARCHAR(32) DEFAULT 'local',
		storage_path VARCHAR(512),
		enabled TINYINT(1) DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_backup_policies_instance (instance_id),
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS backup_records (
		id VARCHAR(64) PRIMARY KEY,
		policy_id VARCHAR(64),
		instance_id VARCHAR(64) NOT NULL,
		backup_type VARCHAR(32) NOT NULL,
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL,
		status VARCHAR(32) DEFAULT 'pending',
		file_path VARCHAR(512),
		file_size BIGINT,
		checksum VARCHAR(128),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_backup_records_instance (instance_id),
		FOREIGN KEY (policy_id) REFERENCES backup_policies(id) ON DELETE CASCADE,
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS restore_records (
		id VARCHAR(64) PRIMARY KEY,
		backup_id VARCHAR(64),
		target_instance_id VARCHAR(64),
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL,
		status VARCHAR(32) DEFAULT 'pending',
		restore_point TIMESTAMP NULL,
		restored_tables TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (backup_id) REFERENCES backup_records(id) ON DELETE CASCADE,
		FOREIGN KEY (target_instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(64) PRIMARY KEY,
		username VARCHAR(64) UNIQUE NOT NULL,
		password VARCHAR(255) NOT NULL,
		email VARCHAR(128) UNIQUE,
		role VARCHAR(32) NOT NULL DEFAULT 'operator',
		status VARCHAR(16) DEFAULT 'active',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS alert_rules (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(128) NOT NULL,
		metric VARCHAR(128) NOT NULL,
		` + "`condition`" + ` VARCHAR(32) NOT NULL,
		threshold DECIMAL(10,2) NOT NULL,
		duration_seconds INT DEFAULT 60,
		severity VARCHAR(16) NOT NULL,
		notification_channels TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS alert_records (
		id VARCHAR(64) PRIMARY KEY,
		rule_id VARCHAR(64),
		instance_id VARCHAR(64),
		triggered_at TIMESTAMP NOT NULL,
		resolved_at TIMESTAMP NULL,
		status VARCHAR(32) DEFAULT 'firing',
		severity VARCHAR(16) NOT NULL,
		value DECIMAL(10,2),
		message TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_alert_records_instance (instance_id),
		FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE,
		FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS parameter_templates (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) UNIQUE NOT NULL,
		description TEXT,
		category VARCHAR(64) NOT NULL,
		is_preset TINYINT(1) DEFAULT 0,
		created_by VARCHAR(64),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_parameter_templates_category (category),
		INDEX idx_parameter_templates_is_preset (is_preset)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS parameter_template_versions (
		id VARCHAR(64) PRIMARY KEY,
		template_id VARCHAR(64) NOT NULL,
		version VARCHAR(32) NOT NULL,
		description TEXT,
		is_active TINYINT(1) DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_parameter_template_versions_template (template_id),
		FOREIGN KEY (template_id) REFERENCES parameter_templates(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS parameter_template_parameters (
		id VARCHAR(64) PRIMARY KEY,
		template_id VARCHAR(64) NOT NULL,
		version_id VARCHAR(64),
		parameter_name VARCHAR(128) NOT NULL,
		value TEXT NOT NULL,
		data_type VARCHAR(32) NOT NULL,
		min_value TEXT,
		max_value TEXT,
		unit VARCHAR(32),
		description TEXT,
		is_dynamic TINYINT(1) DEFAULT 1,
		is_mandatory TINYINT(1) DEFAULT 0,
		category VARCHAR(64),
		INDEX idx_parameter_template_parameters_template (template_id),
		INDEX idx_parameter_template_parameters_version (version_id),
		INDEX idx_parameter_template_parameters_name (parameter_name),
		INDEX idx_parameter_template_parameters_category (category),
		FOREIGN KEY (template_id) REFERENCES parameter_templates(id) ON DELETE CASCADE,
		FOREIGN KEY (version_id) REFERENCES parameter_template_versions(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS hosts (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		address VARCHAR(255) NOT NULL,
		ssh_port INT DEFAULT 22,
		ssh_user VARCHAR(64) NOT NULL,
		ssh_auth_method VARCHAR(16) DEFAULT 'password',
		ssh_credential TEXT,
		os_type VARCHAR(32) DEFAULT 'linux',
		description TEXT,
		tags VARCHAR(512),
		status VARCHAR(32) DEFAULT 'unknown',
		last_check_at TIMESTAMP NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_hosts_status (status),
		INDEX idx_hosts_address (address)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`ALTER TABLE instances ADD CONSTRAINT fk_instances_host FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE SET NULL`,
}

func RunMigrations(ctx context.Context, conn *sql.Conn) error {
	for _, sqlStmt := range InitialSchema {
		if _, err := conn.ExecContext(ctx, sqlStmt); err != nil {
			return fmt.Errorf("failed to execute migration: %w\nSQL: %s", err, sqlStmt)
		}
	}
	return nil
}
