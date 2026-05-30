package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type MGRExecutor struct{}

func NewMGRExecutor() *MGRExecutor {
	return &MGRExecutor{}
}

type MGRConfig struct {
	GroupName      string   `json:"group_name"`
	GroupSeeds     []string `json:"group_seeds"`
	ReplicateUser  string   `json:"replicate_user"`
	ReplicatePass  string   `json:"replicate_pass"`
	DeployMode     string   `json:"deploy_mode"`
	LocalAddress   string   `json:"local_address"`
	LocalPort      int      `json:"local_port"`
	ServerID       int      `json:"server_id"`
	PrimaryHost    string   `json:"primary_host"`
	PrimaryPort    int      `json:"primary_port"`
}

type GroupMemberStatus struct {
	MemberID       string `json:"member_id"`
	MemberHost     string `json:"member_host"`
	MemberPort     int    `json:"member_port"`
	MemberState    string `json:"member_state"`
	MemberRole     string `json:"member_role"`
	Primary        bool   `json:"primary"`
}

func parseMGRConfig(config map[string]interface{}) MGRConfig {
	mc := MGRConfig{
		GroupName:     "mgr_cluster",
		ReplicateUser: "repl",
		ReplicatePass: "repl123",
		DeployMode:    "single-primary",
		LocalPort:     33061,
		ServerID:      1,
		PrimaryPort:   3306,
	}

	if v, ok := config["group_name"].(string); ok {
		mc.GroupName = v
	}
	if v, ok := config["deploy_mode"].(string); ok {
		mc.DeployMode = v
	}
	if v, ok := config["replicate_user"].(string); ok {
		mc.ReplicateUser = v
	}
	if v, ok := config["replicate_pass"].(string); ok {
		mc.ReplicatePass = v
	}
	if v, ok := config["local_address"].(string); ok {
		mc.LocalAddress = v
	}
	if v, ok := config["local_port"].(int); ok {
		mc.LocalPort = v
	}
	if v, ok := config["server_id"].(int); ok {
		mc.ServerID = v
	}
	if v, ok := config["primary_host"].(string); ok {
		mc.PrimaryHost = v
	}
	if v, ok := config["primary_port"].(int); ok {
		mc.PrimaryPort = v
	}
	if seeds, ok := config["group_seeds"].([]interface{}); ok {
		for _, s := range seeds {
			if seed, ok := s.(string); ok {
				mc.GroupSeeds = append(mc.GroupSeeds, seed)
			}
		}
	}

	return mc
}

func (e *MGRExecutor) DeployMGRSinglePrimary(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMGRConfig(req.Config)

	bootstrapResult := e.bootstrapPrimaryNode(ctx, config)
	if bootstrapResult.Status == "failed" {
		return bootstrapResult, nil
	}

	pluginResult := e.installGroupReplicationPlugin(ctx, config)
	if pluginResult.Status == "failed" {
		return pluginResult, nil
	}

	configResult := e.configureMGRParameters(ctx, config)
	if configResult.Status == "failed" {
		return configResult, nil
	}

	groupResult := e.startGroupReplication(ctx, config)
	if groupResult.Status == "failed" {
		return groupResult, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("MGR single-primary cluster deployed successfully. Group: %s, Primary: %s:%d",
			config.GroupName, config.LocalAddress, config.LocalPort-61+3306),
		Timestamp: time.Now(),
	}, nil
}

func (e *MGRExecutor) DeployMGRMultiPrimary(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMGRConfig(req.Config)

	pluginResult := e.installGroupReplicationPlugin(ctx, config)
	if pluginResult.Status == "failed" {
		return pluginResult, nil
	}

	configResult := e.configureMGRParameters(ctx, config)
	if configResult.Status == "failed" {
		return configResult, nil
	}

	configResult = e.enableMultiPrimaryMode(ctx, config)
	if configResult.Status == "failed" {
		return configResult, nil
	}

	groupResult := e.startGroupReplication(ctx, config)
	if groupResult.Status == "failed" {
		return groupResult, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("MGR multi-primary cluster deployed successfully. Group: %s",
			config.GroupName),
		Timestamp: time.Now(),
	}, nil
}

func (e *MGRExecutor) ConfigureGroupMember(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMGRConfig(req.Config)

	pluginResult := e.installGroupReplicationPlugin(ctx, config)
	if pluginResult.Status == "failed" {
		return pluginResult, nil
	}

	configResult := e.configureMGRParameters(ctx, config)
	if configResult.Status == "failed" {
		return configResult, nil
	}

	joinResult := e.joinGroup(ctx, config)
	if joinResult.Status == "failed" {
		return joinResult, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Group member configured and joined successfully. ServerID: %d, Group: %s",
			config.ServerID, config.GroupName),
		Timestamp: time.Now(),
	}, nil
}

func (e *MGRExecutor) MonitorGroupStatus(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMGRConfig(req.Config)

	statusList, err := e.getGroupMembers(ctx, config)
	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  100,
			Message:   fmt.Sprintf("Failed to monitor group status: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	var memberInfo []string
	healthyCount := 0
	for _, member := range statusList {
		memberInfo = append(memberInfo,
			fmt.Sprintf("%s@%s:%d (%s/%s)",
				member.MemberID, member.MemberHost, member.MemberPort,
				member.MemberState, member.MemberRole))
		if member.MemberState == "ONLINE" {
			healthyCount++
		}
	}

	overallStatus := "healthy"
	if healthyCount < len(statusList) {
		overallStatus = "degraded"
	}
	if healthyCount == 0 {
		overallStatus = "failed"
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    overallStatus,
		Progress:  100,
		Message:   fmt.Sprintf("Group status monitored. Members: %d/%d healthy. Details: %s",
			healthyCount, len(statusList), strings.Join(memberInfo, "; ")),
		Timestamp: time.Now(),
	}, nil
}

func (e *MGRExecutor) installGroupReplicationPlugin(ctx context.Context, config MGRConfig) *TaskResult {
	installSQL := "INSTALL PLUGIN group_replication SONAME 'group_replication.so';"

	cmd := exec.CommandContext(ctx, "mysql",
		"-h", config.LocalAddress,
		"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
		"-u", "root", "-e", installSQL)

	if output, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already installed") {
			return &TaskResult{
				Status:    "failed",
				Progress:  10,
				Message:   fmt.Sprintf("Failed to install group_replication plugin: %v, output: %s", err, string(output)),
				Timestamp: time.Now(),
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  20,
		Message:   "Group replication plugin installed",
		Timestamp: time.Now(),
	}
}

func (e *MGRExecutor) configureMGRParameters(ctx context.Context, config MGRConfig) *TaskResult {
	seeds := strings.Join(config.GroupSeeds, ",")
	if seeds == "" {
		seeds = fmt.Sprintf("%s:%d", config.LocalAddress, config.LocalPort)
	}

	configSQLs := []string{
		fmt.Sprintf("SET GLOBAL group_replication_group_name = '%s';", config.GroupName),
		fmt.Sprintf("SET GLOBAL group_replication_local_address = '%s:%d';", config.LocalAddress, config.LocalPort),
		fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s';", seeds),
		"SET GLOBAL group_replication_bootstrap_group = OFF;",
		"SET GLOBAL group_replication_start_on_boot = ON;",
		"SET GLOBAL group_replication_single_primary_mode = ON;",
		"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF;",
		fmt.Sprintf("SET GLOBAL server_id = %d;", config.ServerID),
		"SET GLOBAL log_bin = ON;",
		"SET GLOBAL binlog_format = ROW;",
		"SET GLOBAL binlog_checksum = NONE;",
		"SET GLOBAL gtid_mode = ON;",
		"SET GLOBAL enforce_gtid_consistency = ON;",
		"SET GLOBAL master_info_repository = TABLE;",
		"SET GLOBAL relay_log_info_repository = TABLE;",
		"SET GLOBAL transaction_write_set_extraction = XXHASH64;",
	}

	for _, sql := range configSQLs {
		cmd := exec.CommandContext(ctx, "mysql",
			"-h", config.LocalAddress,
			"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
			"-u", "root", "-e", sql)

		if output, err := cmd.CombinedOutput(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  30,
				Message:   fmt.Sprintf("Failed to configure MGR parameters: %v, SQL: %s, output: %s", err, sql, string(output)),
				Timestamp: time.Now(),
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  50,
		Message:   "MGR parameters configured successfully",
		Timestamp: time.Now(),
	}
}

func (e *MGRExecutor) enableMultiPrimaryMode(ctx context.Context, config MGRConfig) *TaskResult {
	configSQLs := []string{
		"SET GLOBAL group_replication_single_primary_mode = OFF;",
		"SET GLOBAL group_replication_enforce_update_everywhere_checks = ON;",
	}

	for _, sql := range configSQLs {
		cmd := exec.CommandContext(ctx, "mysql",
			"-h", config.LocalAddress,
			"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
			"-u", "root", "-e", sql)

		if output, err := cmd.CombinedOutput(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  40,
				Message:   fmt.Sprintf("Failed to enable multi-primary mode: %v, output: %s", err, string(output)),
				Timestamp: time.Now(),
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  60,
		Message:   "Multi-primary mode enabled",
		Timestamp: time.Now(),
	}
}

func (e *MGRExecutor) startGroupReplication(ctx context.Context, config MGRConfig) *TaskResult {
	bootstrapSQL := "SET GLOBAL group_replication_bootstrap_group = ON;"
	cmd := exec.CommandContext(ctx, "mysql",
		"-h", config.LocalAddress,
		"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
		"-u", "root", "-e", bootstrapSQL)

	if output, err := cmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  70,
			Message:   fmt.Sprintf("Failed to set bootstrap group: %v, output: %s", err, string(output)),
			Timestamp: time.Now(),
		}
	}

	startSQL := "START GROUP_REPLICATION;"
	cmd = exec.CommandContext(ctx, "mysql",
		"-h", config.LocalAddress,
		"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
		"-u", "root", "-e", startSQL)

	if output, err := cmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("Failed to start group replication: %v, output: %s", err, string(output)),
			Timestamp: time.Now(),
		}
	}

	unsetBootstrapSQL := "SET GLOBAL group_replication_bootstrap_group = OFF;"
	cmd = exec.CommandContext(ctx, "mysql",
		"-h", config.LocalAddress,
		"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
		"-u", "root", "-e", unsetBootstrapSQL)

	if output, err := cmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  85,
			Message:   fmt.Sprintf("Failed to unset bootstrap group: %v, output: %s", err, string(output)),
			Timestamp: time.Now(),
		}
	}

	time.Sleep(3 * time.Second)

	return &TaskResult{
		Status:    "completed",
		Progress:  90,
		Message:   "Group replication started successfully",
		Timestamp: time.Now(),
	}
}

func (e *MGRExecutor) joinGroup(ctx context.Context, config MGRConfig) *TaskResult {
	startSQL := "START GROUP_REPLICATION;"
	cmd := exec.CommandContext(ctx, "mysql",
		"-h", config.LocalAddress,
		"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
		"-u", "root", "-e", startSQL)

	if output, err := cmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("Failed to join group: %v, output: %s", err, string(output)),
			Timestamp: time.Now(),
		}
	}

	time.Sleep(2 * time.Second)

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Successfully joined MGR group: %s", config.GroupName),
		Timestamp: time.Now(),
	}
}

func (e *MGRExecutor) getGroupMembers(ctx context.Context, config MGRConfig) ([]GroupMemberStatus, error) {
	querySQL := "SELECT MEMBER_ID, MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members;"

	cmd := exec.CommandContext(ctx, "mysql",
		"-h", config.LocalAddress,
		"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
		"-u", "root", "-N", "-e", querySQL)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query group members: %v", err)
	}

	var members []GroupMemberStatus
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			var port int
			fmt.Sscanf(fields[2], "%d", &port)

			member := GroupMemberStatus{
				MemberID:    fields[0],
				MemberHost:  fields[1],
				MemberPort:  port,
				MemberState: fields[3],
				MemberRole:  fields[4],
				Primary:     fields[4] == "PRIMARY",
			}
			members = append(members, member)
		}
	}

	return members, nil
}

func (e *MGRExecutor) bootstrapPrimaryNode(ctx context.Context, config MGRConfig) *TaskResult {
	createUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%'; "+
			"GRANT GROUP_REPLICATION_STREAM ON *.* TO '%s'@'%%';",
		config.ReplicateUser, config.ReplicatePass,
		config.ReplicateUser, config.ReplicateUser)

	cmd := exec.CommandContext(ctx, "mysql",
		"-h", config.LocalAddress,
		"-P", fmt.Sprintf("%d", config.LocalPort-61+3306),
		"-u", "root", "-e", createUserSQL)

	if output, err := cmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  5,
			Message:   fmt.Sprintf("Failed to create replication user: %v, output: %s", err, string(output)),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  10,
		Message:   "Primary node bootstrapped with replication user",
		Timestamp: time.Now(),
	}
}