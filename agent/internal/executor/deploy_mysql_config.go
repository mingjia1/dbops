package executor

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Allowed top-level deploy config keys (whitelist).
// Agents MUST reject any key outside this set to prevent config injection.
var allowedDeployConfigKeys = map[string]bool{
	"deploy_mode":                    true,
	"master_host":                    true,
	"master_port":                    true,
	"slave_host":                     true,
	"slave_port":                     true,
	"mysql_user":                     true,
	"mysql_password":                 true,
	"mysql_pass":                     true,
	"replicate_user":                 true,
	"replicate_pass":                 true,
	"replication_user":               true,
	"replication_pass":               true,
	"server_id":                      true,
	"package_url":                    true,
	"package_checksum":               true,
	"checksum":                       true,
	"mysql_version":                  true,
	"group_name":                     true,
	"group_seeds":                    true,
	"local_address":                  true,
	"local_port":                     true,
	"local_addresses":                true,
	"bootstrap":                      true,
	"cluster_name":                   true,
	"nodes":                          true,
	"node_host":                      true,
	"wsrep_port":                     true,
	"sst_method":                     true,
	"data_dir":                       true,
	"datadir":                        true,
	"basedir":                        true,
	"os_user":                        true,
	"vip":                            true,
	"vip_interface":                  true,
	"ping_interval":                  true,
	"ping_retry":                     true,
	"ssh_user":                       true,
	"ssh_passwords":                  true,
	"ssh_private_key":                true,
	"semi_sync_enabled":              true,
	"wsrep_ssl_enabled":              true,
	"pxc_encrypt_cluster_traffic":    true,
	"wsrep_sst_port":                 true,
	"install_type":                   true,
	"is_primary":                     true,
	"seeds":                          true,
	"force":                          true,
	"skip_tools_check":               true,
	"root_password":                  true,
	"repl_user":                      true,
	"repl_pass":                      true,
	"slave_hosts":                    true,
	"slave_ports":                    true,
	"manager_host":                   true,
	"primary_host":                   true,
	"primary_port":                   true,
	"mysql_config":                   true,
	"mysql_port":                     true,
	"host":                           true,
	"port":                           true,
	// Cluster switch / role switch fields
	"cluster_id":                     true,
	"cluster_type":                   true,
	"target_role":                    true,
	"old_master_id":                  true,
	"new_master_id":                  true,
	"new_master_host":                true,
	"new_master_port":                true,
	"new_master_user":                true,
	"new_master_pass":                true,
	"new_master_server_uuid":         true,
	"target_host":                    true,
	"target_port":                    true,
	"target_user":                    true,
	"target_pass":                    true,
	"grace_period_sec":               true,
	"switch_type":                    true,
	// Backup / restore fields
	"backup_type":                    true,
	"backup_method":                  true,
	"target_dir":                     true,
	"mysql_host":                     true,
	"base_backup_path":               true,
	"base_backup_label":              true,
	"backup_path":                    true,
	"database_size":                  true,
	// Instance admin fields
	"action":                         true,
	"username":                       true,
	"user_host":                      true,
	"password":                       true,
	"privileges":                     true,
	"scope":                          true,
	// Health check / version detect
	"target_host":                    true,
	"target_port":                    true,
	"target_user":                    true,
	"target_pass":                    true,
	// Upgrade fields
	"upgrade_type":                   true,
	"current_version":                true,
	"target_version":                 true,
	"migration_type":                 true,
	// Blank host init fields
	"blank_host_init":                true,
}

// Allowed deploy_mode values.
var allowedDeployModes = map[string]bool{
	"single":              true,
	"ha-master":           true,
	"ha-replica":          true,
	"master-slave":        true,
	"mha":                 true,
	"mgr":                 true,
	"mgr-member":          true,
	"mgr-single-primary":  true,
	"single-primary":      true,
	"pxc":                 true,
	"blank-host-init":     true,
	"general-cluster-init": true,
}

// ValidateDeployConfig performs security validation on the entire deploy config.
// It checks for:
// 1. Allowed deploy_mode
// 2. Unknown top-level keys (config injection protection)
// 3. Managed keys that must NOT be overridden
// This is a SECOND line of defense — the backend already validates,
// but the agent must NEVER blindly trust backend-supplied config.
func ValidateDeployConfig(config map[string]interface{}) error {
	// Validate deploy_mode
	if mode, ok := config["deploy_mode"].(string); ok && mode != "" {
		if !allowedDeployModes[mode] {
			return fmt.Errorf("deploy_mode %q is not allowed", mode)
		}
	}

	// Validate top-level keys: reject unknown keys
	for key := range config {
		if !allowedDeployConfigKeys[key] {
			return fmt.Errorf("config key %q is not allowed", key)
		}
	}

	// Validate mysql_config (existing logic)
	if err := validateDeployMySQLConfig(config); err != nil {
		return err
	}

	return nil
}

func validateDeployMySQLConfig(config map[string]interface{}) error {
	mysqlConfig := deployMySQLConfig(config)
	for key := range mysqlConfig {
		if isForbiddenDeployMySQLOption(key) {
			return fmt.Errorf("mysql_config %q is managed by DBOps and cannot be overridden", key)
		}
		if !isAllowedDeployMySQLOption(key) {
			return fmt.Errorf("mysql_config %q is not allowed", key)
		}
	}
	return nil
}

func deployMySQLConfig(config map[string]interface{}) map[string]string {
	raw, ok := config["mysql_config"]
	if !ok || raw == nil {
		return nil
	}
	out := map[string]string{}
	switch values := raw.(type) {
	case map[string]string:
		for key, value := range values {
			if strings.TrimSpace(value) != "" {
				out[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		}
	case map[string]interface{}:
		for key, value := range values {
			if value == nil {
				continue
			}
			text := strings.TrimSpace(fmt.Sprintf("%v", value))
			if text != "" {
				out[strings.TrimSpace(key)] = text
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isAllowedDeployMySQLOption(key string) bool {
	_, ok := map[string]bool{
		"max_connections":                true,
		"innodb_buffer_pool_size":        true,
		"innodb_log_file_size":           true,
		"sync_binlog":                    true,
		"innodb_flush_log_at_trx_commit": true,
		"character_set_server":           true,
		"collation_server":               true,
		"lower_case_table_names":         true,
		"read_only":                      true,
		"super_read_only":                true,
	}[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

func isForbiddenDeployMySQLOption(key string) bool {
	_, ok := map[string]bool{
		"server_id":                       true,
		"port":                            true,
		"datadir":                         true,
		"data_dir":                        true,
		"socket":                          true,
		"pid_file":                        true,
		"pid-file":                        true,
		"log_error":                       true,
		"log-error":                       true,
		"basedir":                         true,
		"gtid_mode":                       true,
		"enforce_gtid_consistency":        true,
		"wsrep_cluster_address":           true,
		"wsrep_node_address":              true,
		"group_replication_group_name":    true,
		"group_replication_local_address": true,
	}[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

func deployMySQLConfigLines(config map[string]string) []string {
	if len(config) == 0 {
		return nil
	}
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, config[key]))
	}
	return lines
}

func deployMySQLConfigSQLs(config map[string]string) []string {
	if len(config) == 0 {
		return nil
	}
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sqls := make([]string, 0, len(keys))
	for _, key := range keys {
		sqls = append(sqls, fmt.Sprintf("SET GLOBAL %s = %s;", key, formatVariableValue(config[key])))
	}
	return sqls
}

func applyDeployMySQLConfig(ctx context.Context, host string, port int, user, pass string, config map[string]string) error {
	for _, sql := range deployMySQLConfigSQLs(config) {
		if out, err := mysqlExecCommand(ctx, host, port, user, pass, sql).CombinedOutput(); err != nil {
			return fmt.Errorf("apply %s failed: %v, output: %s", strings.TrimSuffix(sql, ";"), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
