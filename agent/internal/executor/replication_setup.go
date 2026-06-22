package executor

import (
	"context"
	"fmt"
	"os/exec"
)

type ReplicationSetup struct{}

func NewReplicationSetup() *ReplicationSetup {
	return &ReplicationSetup{}
}

type ReplicationConfig struct {
	MasterHost  string `json:"master_host"`
	MasterPort  int    `json:"master_port"`
	ReplUser    string `json:"repl_user"`
	ReplPass    string `json:"repl_pass"`
	AutoPos     bool   `json:"auto_position"`
}

func (s *ReplicationSetup) SetupReplication(ctx context.Context, host string, port int, adminUser, adminPass string, cfg ReplicationConfig) error {
	if cfg.MasterHost == "" {
		return fmt.Errorf("master_host is required")
	}
	if cfg.MasterPort == 0 {
		cfg.MasterPort = 3306
	}
	if !cfg.AutoPos {
		cfg.AutoPos = true
	}

	autoPos := ""
	if cfg.AutoPos {
		autoPos = ", MASTER_AUTO_POSITION=1"
	}

	changeMaster := fmt.Sprintf(
		"CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s'%s",
		cfg.MasterHost, cfg.MasterPort, cfg.ReplUser, cfg.ReplPass, autoPos,
	)

	queries := []string{
		changeMaster,
		"START SLAVE",
	}

	for _, q := range queries {
		if _, err := runMySQLExec(ctx, host, port, adminUser, adminPass, q); err != nil {
			return fmt.Errorf("replication setup: %w", err)
		}
	}
	return nil
}

func (s *ReplicationSetup) CheckReplicationStatus(ctx context.Context, host string, port int, adminUser, adminPass string) (map[string]string, error) {
	output, err := runMySQLExec(ctx, host, port, adminUser, adminPass,
		"SHOW SLAVE STATUS\\G")
	if err != nil {
		return nil, err
	}

	status := make(map[string]string)
	for _, line := range splitLines(output) {
		parsed := parseKVLine(line)
		for k, v := range parsed {
			status[k] = v
		}
	}
	return status, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func parseKVLine(line string) map[string]string {
	for i := 0; i < len(line)-1; i++ {
		if line[i] == ':' && line[i+1] == ' ' {
			key := trimSpace(line[:i])
			val := trimSpace(line[i+2:])
			return map[string]string{key: val}
		}
	}
	return nil
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}

type MGRSetup struct{}

func NewMGRSetup() *MGRSetup {
	return &MGRSetup{}
}

func (s *MGRSetup) BootstrapGroup(ctx context.Context, host string, port int, adminUser, adminPass, groupName, localAddr string, seeds []string) error {
	seedsStr := ""
	for i, seed := range seeds {
		if i > 0 {
			seedsStr += ","
		}
		seedsStr += seed
	}

	queries := []string{
		fmt.Sprintf("SET GLOBAL group_replication_group_name='%s'", groupName),
		fmt.Sprintf("SET GLOBAL group_replication_local_address='%s'", localAddr),
		fmt.Sprintf("SET GLOBAL group_replication_group_seeds='%s'", seedsStr),
		"SET GLOBAL group_replication_bootstrap_group=ON",
		"START GROUP_REPLICATION",
		"SET GLOBAL group_replication_bootstrap_group=OFF",
	}

	for _, q := range queries {
		if _, err := runMySQLExec(ctx, host, port, adminUser, adminPass, q); err != nil {
			return fmt.Errorf("MGR bootstrap: %w", err)
		}
	}
	return nil
}

func (s *MGRSetup) JoinGroup(ctx context.Context, host string, port int, adminUser, adminPass, groupName, localAddr string, seeds []string) error {
	seedsStr := ""
	for i, seed := range seeds {
		if i > 0 {
			seedsStr += ","
		}
		seedsStr += seed
	}

	queries := []string{
		fmt.Sprintf("SET GLOBAL group_replication_group_name='%s'", groupName),
		fmt.Sprintf("SET GLOBAL group_replication_local_address='%s'", localAddr),
		fmt.Sprintf("SET GLOBAL group_replication_group_seeds='%s'", seedsStr),
		"SET GLOBAL group_replication_start_on_boot=OFF",
		"START GROUP_REPLICATION",
	}

	for _, q := range queries {
		if _, err := runMySQLExec(ctx, host, port, adminUser, adminPass, q); err != nil {
			return fmt.Errorf("MGR join: %w", err)
		}
	}
	return nil
}

func (s *MGRSetup) SetPrimaryMember(ctx context.Context, host string, port int, adminUser, adminPass, memberUUID string) error {
	_, err := runMySQLExec(ctx, host, port, adminUser, adminPass,
		fmt.Sprintf("SELECT group_replication_set_as_primary_member('%s')", memberUUID))
	return err
}

type MHASetup struct{}

func NewMHASetup() *MHASetup {
	return &MHASetup{}
}

func (s *MHASetup) InstallNode(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "yum", "install", "-y", "mha4mysql-node")
	if err := cmd.Run(); err != nil {
		cmd2 := exec.CommandContext(ctx, "apt-get", "install", "-y", "mha4mysql-node")
		return cmd2.Run()
	}
	return nil
}

func (s *MHASetup) InstallManager(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "yum", "install", "-y", "mha4mysql-manager")
	if err := cmd.Run(); err != nil {
		cmd2 := exec.CommandContext(ctx, "apt-get", "install", "-y", "mha4mysql-manager")
		return cmd2.Run()
	}
	return nil
}
