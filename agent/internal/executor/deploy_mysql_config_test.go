package executor

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestValidateDeployMySQLConfigRejectsManagedOption(t *testing.T) {
	err := validateDeployMySQLConfig(map[string]interface{}{
		"mysql_config": map[string]interface{}{"server_id": "12"},
	})

	if err == nil {
		t.Fatal("expected managed mysql_config option to be rejected")
	}
}

func TestExecuteDeployRejectsManagedMySQLConfig(t *testing.T) {
	result, err := NewTaskExecutor().ExecuteDeploy(context.Background(), DeployTaskRequest{
		TaskID: "deploy-invalid-config",
		Config: map[string]interface{}{
			"deploy_mode":    "ha-master",
			"mysql_config":   map[string]interface{}{"server_id": "12"},
			"master_host":    "127.0.0.1",
			"master_port":    3306,
			"mysql_user":     "root",
			"mysql_password": "rootpass",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed result, got %#v", result)
	}
	if !strings.Contains(result.Message, "server_id") {
		t.Fatalf("expected message to mention server_id, got %q", result.Message)
	}
}

func TestDeployMySQLConfigSQLsAreStableAndFormatted(t *testing.T) {
	sqls := deployMySQLConfigSQLs(map[string]string{
		"max_connections":                "512",
		"character_set_server":           "utf8mb4",
		"innodb_flush_log_at_trx_commit": "1",
	})

	want := []string{
		"SET GLOBAL character_set_server = 'utf8mb4';",
		"SET GLOBAL innodb_flush_log_at_trx_commit = 1;",
		"SET GLOBAL max_connections = 512;",
	}
	if !reflect.DeepEqual(want, sqls) {
		t.Fatalf("unexpected sqls\nwant=%v\ngot=%v", want, sqls)
	}
}

func TestBuildPXCConfigContentIncludesAllowedMySQLConfig(t *testing.T) {
	content := buildPXCConfigContent(PXCConfig{
		ClusterName: "pxc-prod",
		Nodes:       []string{"10.0.0.11", "10.0.0.12"},
		MySQLPort:   3306,
		WSREPPort:   4567,
		SSTMethod:   "xtrabackup-v2",
		DataDir:     "/data/mysql/pxc-3306",
		NodeHost:    "10.0.0.11",
		MySQLConfig: map[string]string{"max_connections": "512"},
	})

	if !strings.Contains(content, "max_connections=512") {
		t.Fatalf("expected custom mysql config in PXC content:\n%s", content)
	}
	if !strings.Contains(content, "wsrep_cluster_name=pxc-prod") {
		t.Fatalf("expected existing PXC config to remain present:\n%s", content)
	}
}
