package repositories

import (
	"context"
	"fmt"
	"strings"
)

// InitialSchema MySQL 版本 (与历史保持一致).
// 保留 ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 等 MySQL 专属语法.
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
	`CREATE INDEX IF NOT EXISTS idx_task_logs_task_time ON task_logs(task_id, timestamp)`,

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
		task_id VARCHAR(128),
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL,
		status VARCHAR(32) DEFAULT 'pending',
		message TEXT,
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
			expression TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS alert_templates (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			category VARCHAR(32) NOT NULL,
			metric VARCHAR(128) NOT NULL,
			` + "`condition`" + ` VARCHAR(32) NOT NULL DEFAULT '',
			threshold DECIMAL(10,2) DEFAULT 0,
			expression TEXT,
			duration_seconds INT DEFAULT 60,
			severity VARCHAR(16) NOT NULL,
			message TEXT DEFAULT (''),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
			agent_port INT DEFAULT 9090,
			os_type VARCHAR(32) DEFAULT 'linux',
			description TEXT,
			tags VARCHAR(512),
			status VARCHAR(32) DEFAULT 'unknown',
			agent_last_action VARCHAR(32) DEFAULT '',
			agent_last_result VARCHAR(32) DEFAULT '',
			agent_last_message TEXT,
			agent_last_at TIMESTAMP NULL,
			last_check_at TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_hosts_status (status),
		INDEX idx_hosts_address (address)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS migration_tasks (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(128) NOT NULL,
		source_instance_id VARCHAR(64) NOT NULL,
		target_instance_id VARCHAR(64) NOT NULL,
		strategy VARCHAR(32) NOT NULL,
		status VARCHAR(32) DEFAULT 'pending',
		progress INT DEFAULT 0,
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL,
		config TEXT,
		error TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_migration_source (source_instance_id),
		INDEX idx_migration_target (target_instance_id),
		INDEX idx_migration_status (status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS cluster_deployments (
		id VARCHAR(64) PRIMARY KEY,
		cluster_type VARCHAR(32) NOT NULL,
		name VARCHAR(255) NOT NULL,
		status VARCHAR(32) DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_cluster_deployments_type (cluster_type),
		INDEX idx_cluster_deployments_status (status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`ALTER TABLE instances ADD CONSTRAINT fk_instances_host FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE SET NULL`,

	`CREATE TABLE IF NOT EXISTS notification_channels (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(128) NOT NULL UNIQUE,
		channel_type VARCHAR(32) NOT NULL,
		channel_config TEXT,
		template TEXT,
		is_active TINYINT(1) DEFAULT 1,
		priority INT DEFAULT 1,
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_notification_channels_active (is_active)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS alert_notifications (
		id VARCHAR(64) PRIMARY KEY,
		alert_id VARCHAR(64) NOT NULL,
		channel_type VARCHAR(32) NOT NULL,
		channel_config TEXT,
		sent_at TIMESTAMP NULL,
		status VARCHAR(32) DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_alert_notifications_alert (alert_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS approval_requests (
		id VARCHAR(64) PRIMARY KEY,
		requester_id VARCHAR(64) NOT NULL,
		approver_id VARCHAR(64),
		operation_type VARCHAR(64) NOT NULL,
		resource_type VARCHAR(64) NOT NULL,
		resource_id VARCHAR(64) NOT NULL,
		request_reason TEXT,
		approval_status VARCHAR(32) DEFAULT 'pending',
		approval_comment TEXT,
		priority INT DEFAULT 1,
		expires_at TIMESTAMP NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		approved_at TIMESTAMP NULL,
		INDEX idx_approval_requester (requester_id),
		INDEX idx_approval_status (approval_status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS audit_logs (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL,
		operation VARCHAR(64) NOT NULL,
		resource_type VARCHAR(64) NOT NULL,
		resource_id VARCHAR(64),
		action VARCHAR(32) NOT NULL,
		details TEXT,
		result VARCHAR(32) NOT NULL,
		error_msg TEXT,
		ip_address VARCHAR(64),
		user_agent VARCHAR(255),
		prev_hash VARCHAR(64) DEFAULT '',
		hash VARCHAR(64) NOT NULL DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_audit_user (user_id),
		INDEX idx_audit_action (action),
		INDEX idx_audit_created (created_at),
		INDEX idx_audit_hash (hash)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	// Version-agnostic install/upgrade and operational history fields.
	// Keep these ALTER statements idempotent via RunMigrations duplicate-column handling
	// so existing MySQL deployments are upgraded in place.
	`ALTER TABLE instance_connections ADD COLUMN basedir VARCHAR(255) DEFAULT ''`,
	`ALTER TABLE instance_connections ADD COLUMN datadir VARCHAR(255) DEFAULT ''`,
	`ALTER TABLE instance_connections ADD COLUMN os_user VARCHAR(64) DEFAULT ''`,
	`ALTER TABLE instance_connections ADD COLUMN package_url VARCHAR(1024) DEFAULT ''`,
	`ALTER TABLE instance_connections ADD COLUMN version_id VARCHAR(64) DEFAULT ''`,
	`ALTER TABLE instances ADD COLUMN target_version_id VARCHAR(64) DEFAULT ''`,
	`ALTER TABLE backup_records ADD COLUMN message TEXT`,
	`ALTER TABLE backup_records ADD COLUMN task_id VARCHAR(128) DEFAULT ''`,
	`ALTER TABLE alert_rules ADD COLUMN expression TEXT`,
	`CREATE TABLE IF NOT EXISTS escalation_rules (
			id VARCHAR(64) PRIMARY KEY,
			rule_id VARCHAR(64) NOT NULL,
			level INT NOT NULL DEFAULT 0,
			severity VARCHAR(16) NOT NULL,
			interval_sec INT NOT NULL DEFAULT 300,
			notify_channels TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_escalation_rule (rule_id),
			FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS alert_silences (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			rule_ids TEXT DEFAULT (''),
			match_expr TEXT DEFAULT (''),
			start_at TIMESTAMP NOT NULL,
			end_at TIMESTAMP NOT NULL,
			enabled TINYINT(1) DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_alert_silences_enabled (enabled)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS inspection_templates (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			category VARCHAR(32) NOT NULL,
			config TEXT,
			schedule VARCHAR(128) DEFAULT '',
			recipients TEXT DEFAULT (''),
			enabled TINYINT(1) DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_inspection_category (category)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS inspection_reports (
			id VARCHAR(64) PRIMARY KEY,
			template_id VARCHAR(64) NOT NULL,
			instance_id VARCHAR(64) NOT NULL,
			status VARCHAR(32) DEFAULT 'generating',
			summary TEXT DEFAULT (''),
			details TEXT,
			score INT DEFAULT 0,
			generated_at TIMESTAMP NOT NULL,
			sent_at TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_inspection_template (template_id),
			INDEX idx_inspection_instance (instance_id),
			INDEX idx_inspection_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS fault_templates (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			category VARCHAR(32) NOT NULL,
			description TEXT DEFAULT (''),
			fault_type VARCHAR(64) NOT NULL,
			params TEXT,
			duration_sec INT DEFAULT 30,
			severity VARCHAR(16) DEFAULT 'warning',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_fault_category (category)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS fault_executions (
			id VARCHAR(64) PRIMARY KEY,
			template_id VARCHAR(64),
			drill_id VARCHAR(64),
			target_type VARCHAR(32) NOT NULL,
			target_id VARCHAR(64) NOT NULL,
			fault_type VARCHAR(64) NOT NULL,
			params TEXT,
			status VARCHAR(32) DEFAULT 'pending',
			result TEXT,
			started_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP NULL,
			rollback_at TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_fault_drill (drill_id),
			INDEX idx_fault_status (status),
			INDEX idx_fault_target (target_type, target_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS ha_drills (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			description TEXT DEFAULT (''),
			plan TEXT,
			status VARCHAR(32) DEFAULT 'draft',
			result TEXT,
			score INT DEFAULT 0,
			started_at TIMESTAMP NULL,
			completed_at TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_ha_drill_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS ha_drill_reports (
			id VARCHAR(64) PRIMARY KEY,
			drill_id VARCHAR(64) UNIQUE NOT NULL,
			summary TEXT DEFAULT (''),
			timeline TEXT,
			findings TEXT,
			score INT DEFAULT 0,
			generated_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_ha_drill_report (drill_id),
			FOREIGN KEY (drill_id) REFERENCES ha_drills(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS diagnosis_records (
			id VARCHAR(64) PRIMARY KEY,
			instance_id VARCHAR(64) NOT NULL,
			status VARCHAR(32) DEFAULT 'completed',
			summary TEXT DEFAULT (''),
			details TEXT,
			score INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_diagnosis_instance (instance_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS sql_advices (
			id VARCHAR(64) PRIMARY KEY,
			sql_text TEXT NOT NULL,
			` + "`explain`" + ` TEXT DEFAULT (''),
			advice TEXT,
			score INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS role_switch_history (
		id VARCHAR(64) PRIMARY KEY,
		cluster_id VARCHAR(64) NOT NULL,
		cluster_type VARCHAR(32) NOT NULL DEFAULT 'mha',
		instance_id VARCHAR(64) NOT NULL DEFAULT '',
		instance_host VARCHAR(255) NOT NULL DEFAULT '',
		old_role VARCHAR(32) NOT NULL DEFAULT '',
		new_role VARCHAR(32) NOT NULL DEFAULT '',
		old_master_id VARCHAR(64) NOT NULL DEFAULT '',
		new_master_id VARCHAR(64) NOT NULL DEFAULT '',
		rebuilt_replicas TEXT,
		status VARCHAR(32) NOT NULL DEFAULT 'completed',
		message TEXT,
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_rsh_cluster (cluster_id, created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS failover_history (
		id VARCHAR(64) PRIMARY KEY,
		cluster_id VARCHAR(64) NOT NULL,
		old_master_id VARCHAR(64) NOT NULL DEFAULT '',
		new_master_id VARCHAR(64) NOT NULL DEFAULT '',
		failover_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status VARCHAR(32) NOT NULL DEFAULT '',
		success TINYINT(1) DEFAULT 0,
		error_message TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_failover_history_cluster (cluster_id, failover_time)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS key_versions (
		id VARCHAR(64) PRIMARY KEY,
		key_digest VARCHAR(64) NOT NULL,
		version INT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		note TEXT,
		INDEX idx_key_ver (version)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS licenses (
		id VARCHAR(64) PRIMARY KEY,
		tier VARCHAR(16) NOT NULL,
		license_key VARCHAR(128) NOT NULL,
		issued_to VARCHAR(255) NOT NULL,
		issued_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		max_nodes INT DEFAULT 5,
		license_json TEXT NOT NULL,
		active TINYINT(1) DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_licenses_active (active)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	// H10: persist compatibility check results for history viewing.
	`CREATE TABLE IF NOT EXISTS compatibility_checks (
		id VARCHAR(64) PRIMARY KEY,
		instance_id VARCHAR(64) NOT NULL,
		source_version VARCHAR(64) NOT NULL,
		target_version VARCHAR(64) NOT NULL,
		check_type VARCHAR(32) NOT NULL DEFAULT 'full',
		status VARCHAR(32) NOT NULL DEFAULT 'completed',
		is_compatible TINYINT(1) DEFAULT 1,
		warning_count INT DEFAULT 0,
		error_count INT DEFAULT 0,
		incompatibility_list TEXT,
		checked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_compatibility_checks_instance (instance_id),
		INDEX idx_compatibility_checks_compatible (is_compatible)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	// Phase 4: Extend cluster_deployments with full request/plan/status payload.
	`ALTER TABLE cluster_deployments ADD COLUMN request_json TEXT DEFAULT ('')`,
	`ALTER TABLE cluster_deployments ADD COLUMN plan_json TEXT DEFAULT ('')`,
	`ALTER TABLE cluster_deployments ADD COLUMN custom_json TEXT DEFAULT ('')`,
	`ALTER TABLE cluster_deployments ADD COLUMN started_at TIMESTAMP NULL`,
	`ALTER TABLE cluster_deployments ADD COLUMN finished_at TIMESTAMP NULL`,
	`ALTER TABLE cluster_deployments ADD COLUMN error_message TEXT DEFAULT ('')`,

	// Phase 4b: Extend hosts with Agent metadata fields.
	`ALTER TABLE hosts ADD COLUMN agent_version VARCHAR(64) DEFAULT ('')`,
	`ALTER TABLE hosts ADD COLUMN agent_status VARCHAR(32) DEFAULT 'unknown'`,
	`ALTER TABLE hosts ADD COLUMN agent_installed_at TIMESTAMP NULL`,
	`ALTER TABLE hosts ADD COLUMN agent_last_heartbeat TIMESTAMP NULL`,
	`ALTER TABLE hosts ADD COLUMN agent_last_action VARCHAR(32) DEFAULT ''`,
	`ALTER TABLE hosts ADD COLUMN agent_last_result VARCHAR(32) DEFAULT ''`,
	`ALTER TABLE hosts ADD COLUMN agent_last_message TEXT`,
	`ALTER TABLE hosts ADD COLUMN agent_last_at TIMESTAMP NULL`,

	// Phase 4c: Extend cluster_deployments with cluster base info fields.
	`ALTER TABLE cluster_deployments ADD COLUMN cluster_id VARCHAR(64) DEFAULT ('')`,
	`ALTER TABLE cluster_deployments ADD COLUMN display_name VARCHAR(255) DEFAULT ('')`,
	`ALTER TABLE cluster_deployments ADD COLUMN arch VARCHAR(32) DEFAULT ('')`,
	`ALTER TABLE cluster_deployments ADD COLUMN nodes INT DEFAULT 0`,
	`ALTER TABLE cluster_deployments ADD COLUMN mysql_version VARCHAR(64) DEFAULT ('')`,
	`ALTER TABLE cluster_deployments ADD COLUMN config_json TEXT DEFAULT ('')`,
	`ALTER TABLE cluster_deployments ADD INDEX idx_cluster_deployments_cluster_id (cluster_id)`,
	`CREATE TABLE IF NOT EXISTS cluster_credentials (
		id VARCHAR(64) PRIMARY KEY,
		cluster_id VARCHAR(64) NOT NULL,
		account_type VARCHAR(32) NOT NULL,
		username VARCHAR(64) NOT NULL,
		password_enc TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		rotated_at TIMESTAMP NULL,
		INDEX idx_cred_cluster (cluster_id),
		UNIQUE INDEX idx_cred_cluster_type (cluster_id, account_type)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	// M2: Deploy workflow state machine tracking.
	`CREATE TABLE IF NOT EXISTS deploy_workflows (
		id VARCHAR(64) PRIMARY KEY,
		cluster_id VARCHAR(64) NOT NULL,
		workflow_type VARCHAR(32) NOT NULL,
		current_phase VARCHAR(64) NOT NULL DEFAULT '',
		completed_phases TEXT DEFAULT (''),
		status VARCHAR(32) DEFAULT 'running',
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_deploy_workflows_cluster (cluster_id),
		INDEX idx_deploy_workflows_status (status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	// M1: Plugin registry table.
	`CREATE TABLE IF NOT EXISTS plugin_registry (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(128) NOT NULL,
		plugin_type VARCHAR(32) NOT NULL,
		version VARCHAR(32) NOT NULL,
		status VARCHAR(16) DEFAULT 'inactive',
		config TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE INDEX idx_plugin_name (name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	// Phase 6: Topology change event tracking.
	`CREATE TABLE IF NOT EXISTS topology_events (
		id VARCHAR(64) PRIMARY KEY,
		cluster_id VARCHAR(64) NOT NULL,
		event_type VARCHAR(64) NOT NULL,
		old_master_id VARCHAR(64) DEFAULT '',
		new_master_id VARCHAR(64) DEFAULT '',
		node_id VARCHAR(64) DEFAULT '',
		details TEXT DEFAULT (''),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_topology_events_cluster (cluster_id, created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	// Phase 4: Persist per-node deployment progress (was in-memory only).
	`CREATE TABLE IF NOT EXISTS cluster_deploy_nodes (
		id VARCHAR(64) PRIMARY KEY,
		deployment_id VARCHAR(64) NOT NULL,
		instance_id VARCHAR(64) DEFAULT '',
		host VARCHAR(255) NOT NULL,
		mysql_port INT DEFAULT 3306,
		role VARCHAR(32) NOT NULL DEFAULT '',
		status VARCHAR(32) DEFAULT 'pending',
		current_step VARCHAR(128) DEFAULT '',
		message TEXT DEFAULT (''),
		started_at TIMESTAMP NULL,
		finished_at TIMESTAMP NULL,
		error_message TEXT DEFAULT (''),
		INDEX idx_deploy_nodes_deployment (deployment_id),
		FOREIGN KEY (deployment_id) REFERENCES cluster_deployments(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	// Platform settings: generic key-value store for system configuration.
	`CREATE TABLE IF NOT EXISTS platform_settings (
		` + "`key`" + ` VARCHAR(128) PRIMARY KEY,
		value TEXT NOT NULL DEFAULT (''),
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`ALTER TABLE users ADD COLUMN display_name VARCHAR(128) DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN phone VARCHAR(32) DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN source VARCHAR(32) DEFAULT 'local'`,
	`ALTER TABLE users ADD COLUMN last_login_at TIMESTAMP NULL`,
	`ALTER TABLE users ADD COLUMN last_login_ip VARCHAR(64) DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN password_changed_at TIMESTAMP NULL`,
	`CREATE TABLE IF NOT EXISTS roles (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(64) NOT NULL UNIQUE,
		display_name VARCHAR(128) DEFAULT '',
		description TEXT,
		permissions TEXT NOT NULL,
		is_builtin TINYINT(1) DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS user_roles (
		user_id VARCHAR(64) NOT NULL,
		role_id VARCHAR(64) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, role_id),
		INDEX idx_user_roles_user (user_id),
		INDEX idx_user_roles_role (role_id),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}

// schemaSQLite 给 SQLite 用, 去掉了 ENGINE / CHARSET 专属子句.
// 注意: ALTER TABLE ... ADD CONSTRAINT FOREIGN KEY 在 SQLite 中需要用不同语法,
// 改为表内 FK 即可.
var schemaSQLite = []string{
	`CREATE TABLE IF NOT EXISTS instances (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		cluster_id TEXT,
		host_id TEXT REFERENCES hosts(id) ON DELETE SET NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_instances_cluster ON instances(cluster_id)`,
	`CREATE INDEX IF NOT EXISTS idx_instances_host ON instances(host_id)`,

	`CREATE TABLE IF NOT EXISTS instance_connections (
		id TEXT PRIMARY KEY,
		instance_id TEXT UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		username TEXT NOT NULL,
		password_encrypted TEXT NOT NULL,
		ssl_enabled INTEGER DEFAULT 0,
		basedir TEXT DEFAULT '',
		datadir TEXT DEFAULT '',
		os_user TEXT DEFAULT '',
		package_url TEXT DEFAULT '',
		version_id TEXT DEFAULT ''
	)`,

	`CREATE TABLE IF NOT EXISTS instance_versions (
		id TEXT PRIMARY KEY,
		instance_id TEXT UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		flavor TEXT NOT NULL,
		version TEXT NOT NULL,
		full_version TEXT NOT NULL,
		release_date DATE,
		eol_date DATE,
		is_lts INTEGER DEFAULT 0,
		features TEXT,
		engines TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS instance_configs (
		id TEXT PRIMARY KEY,
		instance_id TEXT UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		parameter_template_id TEXT,
		parameters TEXT,
		charset TEXT DEFAULT 'utf8mb4',
		collation TEXT DEFAULT 'utf8mb4_general_ci'
	)`,

	`CREATE TABLE IF NOT EXISTS instance_statuses (
		id TEXT PRIMARY KEY,
		instance_id TEXT UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		run_status TEXT DEFAULT 'unknown',
		health_status TEXT DEFAULT 'unknown',
		role TEXT DEFAULT 'unknown',
		replication_status TEXT,
		seconds_behind_master INTEGER DEFAULT -1,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS instance_topologies (
		id TEXT PRIMARY KEY,
		instance_id TEXT UNIQUE NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		cluster_id TEXT,
		master_id TEXT,
		slave_ids TEXT,
		replication_mode TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		task_type TEXT NOT NULL,
		instance_id TEXT REFERENCES instances(id) ON DELETE CASCADE,
		status TEXT DEFAULT 'pending',
		progress INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		error_message TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(task_type)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_instance ON tasks(instance_id)`,

	`CREATE TABLE IF NOT EXISTS task_workflows (
		id TEXT PRIMARY KEY,
		task_id TEXT UNIQUE NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		workflow_id TEXT,
		workflow_definition TEXT,
		current_stage TEXT,
		completed_stages TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS task_checkpoints (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		checkpoint_id TEXT,
		stage TEXT,
		checkpoint_data TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS task_logs (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		log_id TEXT,
		timestamp TIMESTAMP NOT NULL,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		context TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_task_logs_task_time ON task_logs(task_id, timestamp)`,

	`CREATE TABLE IF NOT EXISTS backup_policies (
		id TEXT PRIMARY KEY,
		instance_id TEXT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		backup_type TEXT NOT NULL,
		schedule TEXT NOT NULL,
		retention_days INTEGER DEFAULT 7,
		storage_type TEXT DEFAULT 'local',
		storage_path TEXT,
		enabled INTEGER DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_backup_policies_instance ON backup_policies(instance_id)`,

	`CREATE TABLE IF NOT EXISTS backup_records (
		id TEXT PRIMARY KEY,
		policy_id TEXT REFERENCES backup_policies(id) ON DELETE CASCADE,
		instance_id TEXT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
		backup_type TEXT NOT NULL,
		task_id TEXT,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		status TEXT DEFAULT 'pending',
		message TEXT,
		file_path TEXT,
		file_size INTEGER,
		checksum TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_backup_records_instance ON backup_records(instance_id)`,

	`CREATE TABLE IF NOT EXISTS restore_records (
		id TEXT PRIMARY KEY,
		backup_id TEXT REFERENCES backup_records(id) ON DELETE CASCADE,
		target_instance_id TEXT REFERENCES instances(id) ON DELETE CASCADE,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		status TEXT DEFAULT 'pending',
		restore_point TIMESTAMP,
		restored_tables TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		email TEXT UNIQUE,
		role TEXT NOT NULL DEFAULT 'operator',
		status TEXT DEFAULT 'active',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS alert_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			metric TEXT NOT NULL,
			` + "`condition`" + ` TEXT NOT NULL,
			threshold REAL NOT NULL,
			duration_seconds INTEGER DEFAULT 60,
			severity TEXT NOT NULL,
			notification_channels TEXT,
			expression TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

	`CREATE TABLE IF NOT EXISTS alert_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category TEXT NOT NULL,
			metric TEXT NOT NULL,
			` + "`condition`" + ` TEXT NOT NULL DEFAULT '',
			threshold REAL DEFAULT 0,
			expression TEXT,
			duration_seconds INTEGER DEFAULT 60,
			severity TEXT NOT NULL,
			message TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

	`CREATE TABLE IF NOT EXISTS escalation_rules (
			id TEXT PRIMARY KEY,
			rule_id TEXT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
			level INTEGER NOT NULL DEFAULT 0,
			severity TEXT NOT NULL,
			interval_sec INTEGER NOT NULL DEFAULT 300,
			notify_channels TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_escalation_rule ON escalation_rules(rule_id)`,
	`CREATE TABLE IF NOT EXISTS alert_silences (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			rule_ids TEXT DEFAULT '',
			match_expr TEXT DEFAULT '',
			start_at TIMESTAMP NOT NULL,
			end_at TIMESTAMP NOT NULL,
			enabled INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_alert_silences_enabled ON alert_silences(enabled)`,
	`CREATE TABLE IF NOT EXISTS inspection_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category TEXT NOT NULL,
			config TEXT,
			schedule TEXT DEFAULT '',
			recipients TEXT DEFAULT '',
			enabled INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_inspection_category ON inspection_templates(category)`,
	`CREATE TABLE IF NOT EXISTS inspection_reports (
			id TEXT PRIMARY KEY,
			template_id TEXT NOT NULL,
			instance_id TEXT NOT NULL,
			status TEXT DEFAULT 'generating',
			summary TEXT DEFAULT '',
			details TEXT,
			score INTEGER DEFAULT 0,
			generated_at TIMESTAMP NOT NULL,
			sent_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_inspection_template ON inspection_reports(template_id)`,
	`CREATE INDEX IF NOT EXISTS idx_inspection_instance ON inspection_reports(instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_inspection_status ON inspection_reports(status)`,
	`CREATE TABLE IF NOT EXISTS fault_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category TEXT NOT NULL,
			description TEXT DEFAULT '',
			fault_type TEXT NOT NULL,
			params TEXT,
			duration_sec INTEGER DEFAULT 30,
			severity TEXT DEFAULT 'warning',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_fault_category ON fault_templates(category)`,
	`CREATE TABLE IF NOT EXISTS fault_executions (
			id TEXT PRIMARY KEY,
			template_id TEXT,
			drill_id TEXT,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			fault_type TEXT NOT NULL,
			params TEXT,
			status TEXT DEFAULT 'pending',
			result TEXT,
			started_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP,
			rollback_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_fault_drill ON fault_executions(drill_id)`,
	`CREATE INDEX IF NOT EXISTS idx_fault_status ON fault_executions(status)`,
	`CREATE INDEX IF NOT EXISTS idx_fault_target ON fault_executions(target_type, target_id)`,
	`CREATE TABLE IF NOT EXISTS ha_drills (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			plan TEXT,
			status TEXT DEFAULT 'draft',
			result TEXT,
			score INTEGER DEFAULT 0,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_ha_drill_status ON ha_drills(status)`,
	`CREATE TABLE IF NOT EXISTS ha_drill_reports (
			id TEXT PRIMARY KEY,
			drill_id TEXT UNIQUE NOT NULL REFERENCES ha_drills(id) ON DELETE CASCADE,
			summary TEXT DEFAULT '',
			timeline TEXT,
			findings TEXT,
			score INTEGER DEFAULT 0,
			generated_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_ha_drill_report ON ha_drill_reports(drill_id)`,
	`CREATE TABLE IF NOT EXISTS diagnosis_records (
			id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL,
			status TEXT DEFAULT 'completed',
			summary TEXT DEFAULT '',
			details TEXT,
			score INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_diagnosis_instance ON diagnosis_records(instance_id)`,
	`CREATE TABLE IF NOT EXISTS sql_advices (
			id TEXT PRIMARY KEY,
			sql_text TEXT NOT NULL,
			` + "`explain`" + ` TEXT DEFAULT '',
			advice TEXT,
			score INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

	`CREATE TABLE IF NOT EXISTS alert_records (
		id TEXT PRIMARY KEY,
		rule_id TEXT,
		instance_id TEXT,
		triggered_at TIMESTAMP NOT NULL,
		resolved_at TIMESTAMP,
		status TEXT DEFAULT 'firing',
		severity TEXT NOT NULL,
		value REAL,
		message TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_alert_records_instance ON alert_records(instance_id)`,

	`CREATE TABLE IF NOT EXISTS parameter_templates (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		description TEXT,
		category TEXT NOT NULL,
		is_preset INTEGER DEFAULT 0,
		created_by TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_templates_category ON parameter_templates(category)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_templates_is_preset ON parameter_templates(is_preset)`,

	`CREATE TABLE IF NOT EXISTS parameter_template_versions (
		id TEXT PRIMARY KEY,
		template_id TEXT NOT NULL REFERENCES parameter_templates(id) ON DELETE CASCADE,
		version TEXT NOT NULL,
		description TEXT,
		is_active INTEGER DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_versions_template ON parameter_template_versions(template_id)`,

	`CREATE TABLE IF NOT EXISTS parameter_template_parameters (
		id TEXT PRIMARY KEY,
		template_id TEXT NOT NULL REFERENCES parameter_templates(id) ON DELETE CASCADE,
		version_id TEXT REFERENCES parameter_template_versions(id) ON DELETE CASCADE,
		parameter_name TEXT NOT NULL,
		value TEXT NOT NULL,
		data_type TEXT NOT NULL,
		min_value TEXT,
		max_value TEXT,
		unit TEXT,
		description TEXT,
		is_dynamic INTEGER DEFAULT 1,
		is_mandatory INTEGER DEFAULT 0,
		category TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_parameters_template ON parameter_template_parameters(template_id)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_parameters_version ON parameter_template_parameters(version_id)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_parameters_name ON parameter_template_parameters(parameter_name)`,
	`CREATE INDEX IF NOT EXISTS idx_parameter_template_parameters_category ON parameter_template_parameters(category)`,

	`CREATE TABLE IF NOT EXISTS hosts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		address TEXT NOT NULL,
		ssh_port INTEGER DEFAULT 22,
		ssh_user TEXT NOT NULL,
		ssh_auth_method TEXT DEFAULT 'password',
		ssh_credential TEXT,
		agent_port INTEGER DEFAULT 9090,
			os_type TEXT DEFAULT 'linux',
			description TEXT,
			tags TEXT,
			status TEXT DEFAULT 'unknown',
			agent_last_action TEXT DEFAULT '',
			agent_last_result TEXT DEFAULT '',
			agent_last_message TEXT,
			agent_last_at TIMESTAMP,
			last_check_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_hosts_status ON hosts(status)`,
	`CREATE INDEX IF NOT EXISTS idx_hosts_address ON hosts(address)`,

	`CREATE TABLE IF NOT EXISTS migration_tasks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		source_instance_id TEXT NOT NULL,
		target_instance_id TEXT NOT NULL,
		strategy TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		progress INTEGER DEFAULT 0,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		config TEXT,
		error TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_migration_source ON migration_tasks(source_instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_migration_target ON migration_tasks(target_instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_migration_status ON migration_tasks(status)`,

	`CREATE TABLE IF NOT EXISTS upgrade_plans (
		id TEXT PRIMARY KEY,
		instance_id TEXT NOT NULL,
		source_version TEXT NOT NULL,
		target_version TEXT NOT NULL,
		strategy TEXT NOT NULL,
		status TEXT DEFAULT 'planned',
		pre_check_result TEXT,
		upgrade_path TEXT,
		estimated_time INTEGER DEFAULT 0,
		risk_level TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP,
		completed_at TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_upgrade_plans_instance ON upgrade_plans(instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_upgrade_plans_status ON upgrade_plans(status)`,

	`CREATE TABLE IF NOT EXISTS cluster_deployments (
		id TEXT PRIMARY KEY,
		cluster_type TEXT NOT NULL,
		name TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cluster_deployments_type ON cluster_deployments(cluster_type)`,
	`CREATE INDEX IF NOT EXISTS idx_cluster_deployments_status ON cluster_deployments(status)`,

	`CREATE TABLE IF NOT EXISTS notification_channels (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		channel_type TEXT NOT NULL,
		channel_config TEXT,
		template TEXT,
		is_active INTEGER DEFAULT 1,
		priority INTEGER DEFAULT 1,
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_notification_channels_active ON notification_channels(is_active)`,

	`CREATE TABLE IF NOT EXISTS alert_notifications (
		id TEXT PRIMARY KEY,
		alert_id TEXT NOT NULL,
		channel_type TEXT NOT NULL,
		channel_config TEXT,
		sent_at TIMESTAMP,
		status TEXT DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_alert_notifications_alert ON alert_notifications(alert_id)`,

	`CREATE TABLE IF NOT EXISTS approval_requests (
		id TEXT PRIMARY KEY,
		requester_id TEXT NOT NULL,
		approver_id TEXT,
		operation_type TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		request_reason TEXT,
		approval_status TEXT DEFAULT 'pending',
		approval_comment TEXT,
		priority INTEGER DEFAULT 1,
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		approved_at TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_approval_requester ON approval_requests(requester_id)`,
	`CREATE INDEX IF NOT EXISTS idx_approval_status ON approval_requests(approval_status)`,

	`CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		operation TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT,
		action TEXT NOT NULL,
		details TEXT,
		result TEXT NOT NULL,
		error_msg TEXT,
		ip_address TEXT,
		user_agent TEXT,
		prev_hash TEXT DEFAULT '',
		hash TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)`,
	`ALTER TABLE audit_logs ADD COLUMN prev_hash TEXT DEFAULT ''`,
	`ALTER TABLE audit_logs ADD COLUMN hash TEXT NOT NULL DEFAULT ''`,
	`CREATE INDEX IF NOT EXISTS idx_audit_hash ON audit_logs(hash)`,

	// 版本无关安装/升级字段 (P-generic). idempotent: 已存在则忽略.
	`ALTER TABLE instance_connections ADD COLUMN basedir VARCHAR(255) DEFAULT ''`,
	`ALTER TABLE instance_connections ADD COLUMN datadir VARCHAR(255) DEFAULT ''`,
	`ALTER TABLE instance_connections ADD COLUMN os_user VARCHAR(64) DEFAULT ''`,
	`ALTER TABLE instance_connections ADD COLUMN package_url VARCHAR(1024) DEFAULT ''`,
	`ALTER TABLE instance_connections ADD COLUMN version_id VARCHAR(64) DEFAULT ''`,
	`ALTER TABLE instances ADD COLUMN target_version_id VARCHAR(64) DEFAULT ''`,
	`ALTER TABLE backup_records ADD COLUMN message TEXT`,
	`ALTER TABLE backup_records ADD COLUMN task_id VARCHAR(128) DEFAULT ''`,
	// P1: 之前 SwitchService.history 是 in-memory map, 后端重启就丢,
	// ListRoleSwitchHistory 返空. 现在持久化到 role_switch_history 表.
	`CREATE TABLE IF NOT EXISTS role_switch_history (
		id VARCHAR(64) PRIMARY KEY,
		cluster_id VARCHAR(64) NOT NULL,
		cluster_type VARCHAR(32) NOT NULL DEFAULT 'mha',
		instance_id VARCHAR(64) NOT NULL DEFAULT '',
		instance_host VARCHAR(255) NOT NULL DEFAULT '',
		old_role VARCHAR(32) NOT NULL DEFAULT '',
		new_role VARCHAR(32) NOT NULL DEFAULT '',
		old_master_id VARCHAR(64) NOT NULL DEFAULT '',
		new_master_id VARCHAR(64) NOT NULL DEFAULT '',
		rebuilt_replicas TEXT,
		status VARCHAR(32) NOT NULL DEFAULT 'completed',
		message TEXT,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_rsh_cluster ON role_switch_history(cluster_id, created_at)`,
	`CREATE TABLE IF NOT EXISTS failover_history (
		id VARCHAR(64) PRIMARY KEY,
		cluster_id VARCHAR(64) NOT NULL,
		old_master_id VARCHAR(64) NOT NULL DEFAULT '',
		new_master_id VARCHAR(64) NOT NULL DEFAULT '',
		failover_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status VARCHAR(32) NOT NULL DEFAULT '',
		success INTEGER DEFAULT 0,
		error_message TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_failover_history_cluster ON failover_history(cluster_id, failover_time)`,
	`CREATE TABLE IF NOT EXISTS masking_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			field_path TEXT NOT NULL,
			pattern TEXT DEFAULT '',
			algorithm TEXT NOT NULL DEFAULT 'mask',
			replacement TEXT DEFAULT '',
			roles TEXT NOT NULL DEFAULT '*',
			enabled INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE TABLE IF NOT EXISTS key_versions (
			id TEXT PRIMARY KEY,
			key_digest TEXT NOT NULL,
			version INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			note TEXT
		)`,
	`CREATE INDEX IF NOT EXISTS idx_key_ver ON key_versions(version)`,
	`CREATE TABLE IF NOT EXISTS licenses (
			id TEXT PRIMARY KEY,
			tier TEXT NOT NULL,
			license_key TEXT NOT NULL,
			issued_to TEXT NOT NULL,
			issued_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			max_nodes INTEGER DEFAULT 5,
			license_json TEXT NOT NULL,
			active INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	`CREATE INDEX IF NOT EXISTS idx_licenses_active ON licenses(active)`,

	// H10: persist compatibility check results for history viewing.
	`CREATE TABLE IF NOT EXISTS compatibility_checks (
		id TEXT PRIMARY KEY,
		instance_id TEXT NOT NULL,
		source_version TEXT NOT NULL,
		target_version TEXT NOT NULL,
		check_type TEXT NOT NULL DEFAULT 'full',
		status TEXT NOT NULL DEFAULT 'completed',
		is_compatible INTEGER DEFAULT 1,
		warning_count INTEGER DEFAULT 0,
		error_count INTEGER DEFAULT 0,
		incompatibility_list TEXT,
		checked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_compatibility_checks_instance ON compatibility_checks(instance_id)`,
	`CREATE INDEX IF NOT EXISTS idx_compatibility_checks_compatible ON compatibility_checks(is_compatible)`,

	// Phase 4: Extend cluster_deployments with full request/plan/status payload.
	`ALTER TABLE cluster_deployments ADD COLUMN request_json TEXT DEFAULT ''`,
	`ALTER TABLE cluster_deployments ADD COLUMN plan_json TEXT DEFAULT ''`,
	`ALTER TABLE cluster_deployments ADD COLUMN custom_json TEXT DEFAULT ''`,
	`ALTER TABLE cluster_deployments ADD COLUMN started_at TIMESTAMP`,
	`ALTER TABLE cluster_deployments ADD COLUMN finished_at TIMESTAMP`,
	`ALTER TABLE cluster_deployments ADD COLUMN error_message TEXT DEFAULT ''`,

	// Phase 4b: Extend hosts with Agent metadata fields.
	`ALTER TABLE hosts ADD COLUMN agent_version TEXT DEFAULT ''`,
	`ALTER TABLE hosts ADD COLUMN agent_status TEXT DEFAULT 'unknown'`,
	`ALTER TABLE hosts ADD COLUMN agent_installed_at TIMESTAMP`,
	`ALTER TABLE hosts ADD COLUMN agent_last_heartbeat TIMESTAMP`,
	`ALTER TABLE hosts ADD COLUMN agent_last_action TEXT DEFAULT ''`,
	`ALTER TABLE hosts ADD COLUMN agent_last_result TEXT DEFAULT ''`,
	`ALTER TABLE hosts ADD COLUMN agent_last_message TEXT`,
	`ALTER TABLE hosts ADD COLUMN agent_last_at TIMESTAMP`,

	// Phase 4c: Extend cluster_deployments with cluster base info fields.
	`ALTER TABLE cluster_deployments ADD COLUMN cluster_id TEXT DEFAULT ''`,
	`ALTER TABLE cluster_deployments ADD COLUMN display_name TEXT DEFAULT ''`,
	`ALTER TABLE cluster_deployments ADD COLUMN arch TEXT DEFAULT ''`,
	`ALTER TABLE cluster_deployments ADD COLUMN nodes INTEGER DEFAULT 0`,
	`ALTER TABLE cluster_deployments ADD COLUMN mysql_version TEXT DEFAULT ''`,
	`ALTER TABLE cluster_deployments ADD COLUMN config_json TEXT DEFAULT ''`,
	`CREATE INDEX IF NOT EXISTS idx_cluster_deployments_cluster_id ON cluster_deployments(cluster_id)`,
	`CREATE TABLE IF NOT EXISTS cluster_credentials (
		id TEXT PRIMARY KEY,
		cluster_id TEXT NOT NULL,
		account_type TEXT NOT NULL,
		username TEXT NOT NULL,
		password_enc TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		rotated_at TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cred_cluster ON cluster_credentials(cluster_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_cred_cluster_type ON cluster_credentials(cluster_id, account_type)`,

	// M2: Deploy workflow state machine tracking.
	`CREATE TABLE IF NOT EXISTS deploy_workflows (
		id TEXT PRIMARY KEY,
		cluster_id TEXT NOT NULL,
		workflow_type TEXT NOT NULL,
		current_phase TEXT NOT NULL DEFAULT '',
		completed_phases TEXT DEFAULT '',
		status TEXT DEFAULT 'running',
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_deploy_workflows_cluster ON deploy_workflows(cluster_id)`,
	`CREATE INDEX IF NOT EXISTS idx_deploy_workflows_status ON deploy_workflows(status)`,

	// M1: Plugin registry table.
	`CREATE TABLE IF NOT EXISTS plugin_registry (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		plugin_type TEXT NOT NULL,
		version TEXT NOT NULL,
		status TEXT DEFAULT 'inactive',
		config TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_plugin_name ON plugin_registry(name)`,

	// Phase 6: Topology change event tracking.
	`CREATE TABLE IF NOT EXISTS topology_events (
		id TEXT PRIMARY KEY,
		cluster_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		old_master_id TEXT DEFAULT '',
		new_master_id TEXT DEFAULT '',
		node_id TEXT DEFAULT '',
		details TEXT DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_topology_events_cluster ON topology_events(cluster_id, created_at)`,

	// Phase 4: Persist per-node deployment progress (was in-memory only).
	`CREATE TABLE IF NOT EXISTS cluster_deploy_nodes (
			id TEXT PRIMARY KEY,
			deployment_id TEXT NOT NULL REFERENCES cluster_deployments(id) ON DELETE CASCADE,
			instance_id TEXT DEFAULT '',
			host TEXT NOT NULL,
			mysql_port INTEGER DEFAULT 3306,
			role TEXT NOT NULL DEFAULT '',
			status TEXT DEFAULT 'pending',
			current_step TEXT DEFAULT '',
			message TEXT DEFAULT '',
			started_at TIMESTAMP,
			finished_at TIMESTAMP,
			error_message TEXT DEFAULT ''
		)`,
	`CREATE INDEX IF NOT EXISTS idx_deploy_nodes_deployment ON cluster_deploy_nodes(deployment_id)`,

	// Platform settings: generic key-value store for system configuration.
	`CREATE TABLE IF NOT EXISTS platform_settings (
		key VARCHAR(128) PRIMARY KEY,
		value TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`ALTER TABLE users ADD COLUMN display_name TEXT DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN phone TEXT DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN source TEXT DEFAULT 'local'`,
	`ALTER TABLE users ADD COLUMN last_login_at TIMESTAMP`,
	`ALTER TABLE users ADD COLUMN last_login_ip TEXT DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN password_changed_at TIMESTAMP`,
	`CREATE TABLE IF NOT EXISTS roles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		display_name TEXT DEFAULT '',
		description TEXT,
		permissions TEXT NOT NULL,
		is_builtin INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS user_roles (
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, role_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_user_roles_user ON user_roles(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_user_roles_role ON user_roles(role_id)`,
}

// SchemaFor 按方言返回对应 schema. 这是给 main.go 调用的统一入口.
func SchemaFor(d Dialect) []string {
	if d == DialectSQLite {
		return schemaSQLite
	}
	return InitialSchema
}

// RunMigrations 根据连接所属方言执行对应 schema.
func RunMigrations(ctx context.Context, db *Database) error {
	if db == nil || db.Pool == nil {
		return fmt.Errorf("nil database")
	}
	stmts := SchemaFor(db.Dialect)
	for _, s := range stmts {
		if _, err := db.Pool.ExecContext(ctx, s); err != nil {
			// ALTER TABLE ... ADD CONSTRAINT 在已有表上重复执行会报错, 忽略.
			if isAlreadyExistsError(err) {
				continue
			}
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, trimSQL(s))
		}
	}
	return nil
}

func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// P1-4: 覆盖更多 MySQL 错误码以保证 ALTER / FK 重复执行安全.
	// 1022 Duplicate entry for key, 1050 Table already exists, 1060 Duplicate column name,
	// 1061 Duplicate key name, 1062 Duplicate entry, 1091 Can't DROP (idempotent),
	// 1826 Duplicate foreign key constraint name.
	if strings.Contains(msg, "error 1022") ||
		strings.Contains(msg, "error 1050") ||
		strings.Contains(msg, "error 1060") ||
		strings.Contains(msg, "error 1061") ||
		strings.Contains(msg, "error 1062") ||
		strings.Contains(msg, "error 1091") ||
		strings.Contains(msg, "error 1826") {
		return true
	}
	// 兜底关键字匹配, 兼容 driver 版本差异 (大小写无关).
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "table") && strings.Contains(msg, "already exists")
}

func trimSQL(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
