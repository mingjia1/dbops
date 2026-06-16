package executor

import (
	"context"
	"testing"
)

func TestValidateDeployConfig_AcceptsValidConfig(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode":    "ha-master",
		"master_host":    "10.0.0.11",
		"master_port":    3306,
		"mysql_user":     "root",
		"mysql_password": "rootpass",
		"replicate_user": "repl",
		"replicate_pass": "replpass",
		"package_url":    "https://repo.example/mysql.tar.gz",
		"mysql_config":   map[string]interface{}{"max_connections": "512"},
	})
	if err != nil {
		t.Fatalf("expected valid config to pass, got: %v", err)
	}
}

func TestValidateDeployConfig_RejectsUnknownKey(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode":       "single",
		"malicious_option": "DROP TABLE mysql.user",
	})
	if err == nil {
		t.Fatal("expected unknown key to be rejected")
	}
}

func TestValidateDeployConfig_RejectsInvalidDeployMode(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode": "invalid-mode-xyz",
	})
	if err == nil {
		t.Fatal("expected invalid deploy_mode to be rejected")
	}
}

func TestValidateDeployConfig_RejectsManagedMySQLOption(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode":  "ha-master",
		"mysql_config": map[string]interface{}{"server_id": "12"},
	})
	if err == nil {
		t.Fatal("expected managed mysql_config option to be rejected")
	}
}

func TestValidateDeployConfig_RejectsDisallowedMySQLOption(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode":  "ha-master",
		"mysql_config": map[string]interface{}{"skip_grant_tables": "ON"},
	})
	if err == nil {
		t.Fatal("expected disallowed mysql_config option to be rejected")
	}
}

func TestValidateDeployConfig_AllowsMGRConfig(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode":    "mgr",
		"group_name":     "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"group_seeds":    []interface{}{"10.0.0.11:33061", "10.0.0.12:33061"},
		"local_address":  "10.0.0.11",
		"local_port":     33061,
		"mysql_port":     3306,
		"server_id":      1,
		"primary_host":   "10.0.0.11",
		"primary_port":   3306,
		"bootstrap":      true,
		"replicate_user": "repl",
		"replicate_pass": "replpass",
		"mysql_user":     "root",
		"mysql_password": "rootpass",
	})
	if err != nil {
		t.Fatalf("expected mgr config to pass, got: %v", err)
	}
}

func TestValidateDeployConfig_AllowsPXCConfig(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode":  "pxc",
		"cluster_name": "pxc-prod",
		"bootstrap":    true,
		"nodes":        []interface{}{"10.0.0.11", "10.0.0.12"},
		"node_host":    "10.0.0.11",
		"mysql_port":   3306,
		"wsrep_port":   4567,
		"sst_method":   "xtrabackup-v2",
		"data_dir":     "/data/mysql/pxc-3306",
		"mysql_user":   "root",
		"mysql_password": "rootpass",
	})
	if err != nil {
		t.Fatalf("expected pxc config to pass, got: %v", err)
	}
}

func TestValidateDeployConfig_AllowsMHAConfig(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode":  "mha",
		"manager_host": "10.0.0.10",
		"master_host":  "10.0.0.11",
		"master_port":  3306,
		"slave_hosts":  []interface{}{"10.0.0.12"},
		"slave_ports":  []interface{}{3306},
		"vip":          "10.0.0.100",
		"repl_user":    "repl",
		"repl_pass":    "replpass",
		"mysql_user":   "root",
		"mysql_password": "rootpass",
		"ssh_user":     "dbops",
	})
	if err != nil {
		t.Fatalf("expected mha config to pass, got: %v", err)
	}
}

func TestValidateDeployConfig_AllowsBlankHostInit(t *testing.T) {
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode":     "blank-host-init",
		"mysql_version":   "8.0.36",
		"mysql_port":      3306,
		"data_dir":        "/data/mysql/3306",
		"basedir":         "/opt/mysql",
		"root_password":   "rootpass",
		"package_url":     "https://repo.example/mysql.tar.gz",
		"package_checksum": "abc123",
	})
	if err != nil {
		t.Fatalf("expected blank-host-init config to pass, got: %v", err)
	}
}

func TestValidateDeployConfig_AcceptsNonIPNodeValues(t *testing.T) {
	// Agent-level validation does not validate IP format of node values;
	// that's the backend's responsibility. Injection strings in nodes
	// will be caught at execution time.
	err := ValidateDeployConfig(map[string]interface{}{
		"deploy_mode": "pxc",
		"nodes":       []interface{}{"$(cat /etc/shadow)", "10.0.0.12"},
	})
	if err != nil {
		t.Fatalf("expected nodes with non-IP to pass validation (agent does not validate IP format), got: %v", err)
	}
}

func TestExecuteDeployRejectsUnknownConfigKey(t *testing.T) {
	result, err := NewTaskExecutor().ExecuteDeploy(context.Background(), DeployTaskRequest{
		TaskID: "reject-unknown-key",
		Config: map[string]interface{}{
			"deploy_mode":       "ha-master",
			"__proto__":         "malicious",
			"master_host":       "127.0.0.1",
			"master_port":       3306,
			"mysql_user":        "root",
			"mysql_password":    "rootpass",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed result, got status=%q msg=%q", result.Status, result.Message)
	}
}
