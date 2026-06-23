package executor

import (
	"context"
	"fmt"
)

type PXCSetup struct{}

func NewPXCSetup() *PXCSetup {
	return &PXCSetup{}
}

type PXCNodeConfig struct {
	Action          string   `json:"action"`
	WsrepClusterAddr string  `json:"wsrep_cluster_addr"`
	NodeAddress     string   `json:"node_address"`
	ClusterName     string   `json:"cluster_name"`
	SSTMethod       string   `json:"sst_method"`
}

func (s *PXCSetup) BootstrapNode(ctx context.Context, host string, port int, adminUser, adminPass string, cfg PXCNodeConfig) error {
	if cfg.NodeAddress == "" {
		return fmt.Errorf("node_address is required")
	}
	if cfg.SSTMethod == "" {
		cfg.SSTMethod = "xtrabackup-v2"
	}

	queries := []string{
		"SET GLOBAL wsrep_on=ON",
		fmt.Sprintf("SET GLOBAL wsrep_cluster_address='gcomm://'", ),
		fmt.Sprintf("SET GLOBAL wsrep_node_address='%s'", cfg.NodeAddress),
		fmt.Sprintf("SET GLOBAL wsrep_sst_method='%s'", cfg.SSTMethod),
		"SET GLOBAL wsrep_cluster_name='pxc_cluster'",
	}

	for _, q := range queries {
		if _, err := runMySQLExec(ctx, host, port, adminUser, adminPass, q); err != nil {
			return fmt.Errorf("PXC bootstrap: %w", err)
		}
	}
	return nil
}

func (s *PXCSetup) JoinCluster(ctx context.Context, host string, port int, adminUser, adminPass string, cfg PXCNodeConfig) error {
	if cfg.WsrepClusterAddr == "" {
		return fmt.Errorf("wsrep_cluster_addr is required")
	}
	if cfg.NodeAddress == "" {
		return fmt.Errorf("node_address is required")
	}
	if cfg.SSTMethod == "" {
		cfg.SSTMethod = "xtrabackup-v2"
	}

	queries := []string{
		"SET GLOBAL wsrep_on=ON",
		fmt.Sprintf("SET GLOBAL wsrep_cluster_address='%s'", cfg.WsrepClusterAddr),
		fmt.Sprintf("SET GLOBAL wsrep_node_address='%s'", cfg.NodeAddress),
		fmt.Sprintf("SET GLOBAL wsrep_sst_method='%s'", cfg.SSTMethod),
	}

	for _, q := range queries {
		if _, err := runMySQLExec(ctx, host, port, adminUser, adminPass, q); err != nil {
			return fmt.Errorf("PXC join: %w", err)
		}
	}
	return nil
}

func (s *PXCSetup) CheckClusterStatus(ctx context.Context, host string, port int, adminUser, adminPass string) (map[string]string, error) {
	status := make(map[string]string)

	size, err := runMySQLExec(ctx, host, port, adminUser, adminPass,
		"SHOW STATUS LIKE 'wsrep_cluster_size'")
	if err != nil {
		return nil, fmt.Errorf("check cluster size: %w", err)
	}
	status["wsrep_cluster_size"] = size

	state, err := runMySQLExec(ctx, host, port, adminUser, adminPass,
		"SHOW STATUS LIKE 'wsrep_local_state_comment'")
	if err != nil {
		return nil, fmt.Errorf("check local state: %w", err)
	}
	status["wsrep_local_state_comment"] = state

	return status, nil
}
