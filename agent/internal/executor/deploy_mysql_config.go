package executor

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

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
