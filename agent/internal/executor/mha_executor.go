package executor

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

type MHAExecutor struct{}

func NewMHAExecutor() *MHAExecutor {
	return &MHAExecutor{}
}

type MHAConfig struct {
	ManagerHost   string   `json:"manager_host"`
	MasterHost    string   `json:"master_host"`
	MasterPort    int      `json:"master_port"`
	SlaveHosts    []string `json:"slave_hosts"`
	SlavePorts    []int    `json:"slave_ports"`
	VIP           string   `json:"vip"`
	VIPInterface  string   `json:"vip_interface"`
	ReplUser      string   `json:"repl_user"`
	ReplPass      string   `json:"repl_pass"`
	ManagerUser   string   `json:"manager_user"`
	ManagerPass   string   `json:"manager_pass"`
	MySQLUser     string   `json:"mysql_user"`
	MySQLPassword string   `json:"mysql_password"`
	PingInterval  int      `json:"ping_interval"`
	PingRetry     int      `json:"ping_retry"`
	SSHUser       string   `json:"ssh_user"`
	SSHPrivateKey string   `json:"ssh_private_key"`
}

type MHAManagerConfig struct {
	ConfigFile     string   `json:"config_file"`
	ManagerHost    string   `json:"manager_host"`
	MasterHost     string   `json:"master_host"`
	MasterPort     int      `json:"master_port"`
	CandidateHosts []string `json:"candidate_hosts"`
	VIP            string   `json:"vip"`
	VIPInterface   string   `json:"vip_interface"`
	ReplUser       string   `json:"repl_user"`
	ReplPass       string   `json:"repl_pass"`
	SSHUser        string   `json:"ssh_user"`
	WorkDir        string   `json:"work_dir"`
	PingInterval   int      `json:"ping_interval"`
}

type MHANodeConfig struct {
	NodeHost      string `json:"node_host"`
	NodePort      int    `json:"node_port"`
	SSHUser       string `json:"ssh_user"`
	SSHPrivateKey string `json:"ssh_private_key"`
	ReplUser      string `json:"repl_user"`
	ReplPass      string `json:"repl_pass"`
	InstallDir    string `json:"install_dir"`
}

type MasterFailureInfo struct {
	MasterHost  string    `json:"master_host"`
	MasterPort  int       `json:"master_port"`
	FailureTime time.Time `json:"failure_time"`
	IsReachable bool      `json:"is_reachable"`
	MySQLAlive  bool      `json:"mysql_alive"`
	ReplWorking bool      `json:"repl_working"`
}

type FailoverResult struct {
	OldMaster    string    `json:"old_master"`
	NewMaster    string    `json:"new_master"`
	FailoverTime time.Time `json:"failover_time"`
	VIPSwitched  bool      `json:"vip_switched"`
	Status       string    `json:"status"`
}

type VIPSwitchResult struct {
	VIP          string    `json:"vip"`
	Interface    string    `json:"interface"`
	OldHost      string    `json:"old_host"`
	NewHost      string    `json:"new_host"`
	SwitchTime   time.Time `json:"switch_time"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message"`
}

func parseMHAConfig(config map[string]interface{}) MHAConfig {
	mc := MHAConfig{
		ManagerHost:   "localhost",
		MasterHost:    "localhost",
		MasterPort:    3306,
		VIP:           "192.168.1.100",
		VIPInterface:  "eth0",
		ReplUser:      "repl",
		ReplPass:      "repl123",
		ManagerUser:   "mha_manager",
		ManagerPass:   "mha123",
		MySQLUser:     "root",
		PingInterval:  3,
		PingRetry:     3,
		SSHUser:       "root",
		SSHPrivateKey: "/root/.ssh/id_rsa",
	}

	if v, ok := config["manager_host"].(string); ok {
		mc.ManagerHost = v
	}
	if v, ok := config["master_host"].(string); ok {
		mc.MasterHost = v
	}
	if v := configInt(config, "master_port"); v != 0 {
		mc.MasterPort = v
	}
	if v, ok := config["vip"].(string); ok {
		mc.VIP = v
	}
	if v, ok := config["vip_interface"].(string); ok {
		mc.VIPInterface = v
	}
	if v, ok := config["repl_user"].(string); ok {
		mc.ReplUser = v
	}
	if v, ok := config["repl_pass"].(string); ok {
		mc.ReplPass = v
	}
	if v, ok := config["manager_user"].(string); ok {
		mc.ManagerUser = v
	}
	if v, ok := config["manager_pass"].(string); ok {
		mc.ManagerPass = v
	}
	if v, ok := config["mysql_user"].(string); ok {
		mc.MySQLUser = v
	}
	if v, ok := config["mysql_password"].(string); ok {
		mc.MySQLPassword = v
	}
	if v := configInt(config, "ping_interval"); v != 0 {
		mc.PingInterval = v
	}
	if v := configInt(config, "ping_retry"); v != 0 {
		mc.PingRetry = v
	}
	if v, ok := config["ssh_user"].(string); ok {
		mc.SSHUser = v
	}
	if v, ok := config["ssh_private_key"].(string); ok {
		mc.SSHPrivateKey = v
	}
	if v, ok := config["slave_hosts"].([]string); ok {
		mc.SlaveHosts = v
	} else if hosts, ok := config["slave_hosts"].([]interface{}); ok {
		for _, host := range hosts {
			if s, ok := host.(string); ok {
				mc.SlaveHosts = append(mc.SlaveHosts, s)
			}
		}
	}
	if v, ok := config["slave_ports"].([]int); ok {
		mc.SlavePorts = v
	} else if ports, ok := config["slave_ports"].([]interface{}); ok {
		for _, port := range ports {
			switch n := port.(type) {
			case int:
				mc.SlavePorts = append(mc.SlavePorts, n)
			case float64:
				mc.SlavePorts = append(mc.SlavePorts, int(n))
			}
		}
	}

	return mc
}

func (e *MHAExecutor) DeployMHA(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMHAConfig(req.Config)

	nodeResult := e.installMHANode(ctx, config)
	if nodeResult.Status == "failed" {
		return nodeResult, nil
	}

	managerResult := e.configureMHAManager(ctx, config)
	if managerResult.Status == "failed" {
		return managerResult, nil
	}

	validateResult := e.validateMHADeployment(ctx, config)
	return validateResult, nil
}

func (e *MHAExecutor) installMHANode(ctx context.Context, config MHAConfig) *TaskResult {
	allHosts := append([]string{config.MasterHost}, config.SlaveHosts...)

	for i, host := range allHosts {
		installCmd := mhaShellCommand(ctx, host, config.SSHUser, config.SSHPrivateKey, "yum install -y mha4mysql-node || apt-get install -y mha4mysql-node; command -v save_binary_logs")
		if out, err := installCmd.CombinedOutput(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  10 + i*10,
				Message:   fmt.Sprintf("Failed to install MHA node on %s: %v, output: %s", host, err, strings.TrimSpace(string(out))),
				Timestamp: time.Now(),
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  30,
		Message:   fmt.Sprintf("MHA node installed on %d hosts", len(allHosts)),
		Timestamp: time.Now(),
	}
}

func (e *MHAExecutor) configureMHAManager(ctx context.Context, config MHAConfig) *TaskResult {
	createUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT ALL PRIVILEGES ON *.* TO '%s'@'%%';",
		config.ManagerUser, config.ManagerPass, config.ManagerUser,
	)

	allNodes := []struct {
		host string
		port int
	}{{host: config.MasterHost, port: config.MasterPort}}
	for i, host := range config.SlaveHosts {
		port := config.MasterPort
		if i < len(config.SlavePorts) && config.SlavePorts[i] != 0 {
			port = config.SlavePorts[i]
		}
		allNodes = append(allNodes, struct {
			host string
			port int
		}{host: host, port: port})
	}
	for _, node := range allNodes {
		cmd := mysqlExecCommand(ctx, node.host, node.port, config.MySQLUser, config.MySQLPassword, createUserSQL)
		if out, err := cmd.CombinedOutput(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  35,
				Message:   fmt.Sprintf("Failed to create MHA manager user on %s:%d: %v, output: %s", node.host, node.port, err, strings.TrimSpace(string(out))),
				Timestamp: time.Now(),
			}
		}
	}

	installManagerCmd := mhaShellCommand(ctx, config.ManagerHost, config.SSHUser, config.SSHPrivateKey, "yum install -y mha4mysql-manager || apt-get install -y mha4mysql-manager; command -v masterha_check_ssh && command -v masterha_check_repl")
	if out, err := installManagerCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  40,
			Message:   fmt.Sprintf("Failed to install MHA manager: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	configContent := generateMHAAppConfig(config)
	writeConfigCmd := mhaShellCommand(ctx, config.ManagerHost, config.SSHUser, config.SSHPrivateKey, fmt.Sprintf("mkdir -p /etc/mha /var/log/mha/app1 && cat > /etc/mha/app1.cnf <<'EOF'\n%s\nEOF", configContent))
	if out, err := writeConfigCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  45,
			Message:   fmt.Sprintf("Failed to write MHA manager config: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  50,
		Message:   "MHA manager configured successfully",
		Timestamp: time.Now(),
	}
}

func generateMHAAppConfig(config MHAConfig) string {
	var sb strings.Builder
	sb.WriteString("[server default]\n")
	sb.WriteString(fmt.Sprintf("manager_workdir=/var/log/mha/app1\n"))
	sb.WriteString(fmt.Sprintf("manager_log=/var/log/mha/app1/manager.log\n"))
	sb.WriteString(fmt.Sprintf("user=%s\n", config.ManagerUser))
	sb.WriteString(fmt.Sprintf("password=%s\n", config.ManagerPass))
	sb.WriteString(fmt.Sprintf("repl_user=%s\n", config.ReplUser))
	sb.WriteString(fmt.Sprintf("repl_password=%s\n", config.ReplPass))
	sb.WriteString(fmt.Sprintf("ssh_user=%s\n", config.SSHUser))
	sb.WriteString(fmt.Sprintf("ping_interval=%d\n\n", config.PingInterval))
	sb.WriteString("[server1]\n")
	sb.WriteString(fmt.Sprintf("hostname=%s\n", config.MasterHost))
	sb.WriteString(fmt.Sprintf("port=%d\n", config.MasterPort))
	sb.WriteString("candidate_master=1\n")
	for i, host := range config.SlaveHosts {
		port := config.MasterPort
		if i < len(config.SlavePorts) && config.SlavePorts[i] != 0 {
			port = config.SlavePorts[i]
		}
		sb.WriteString(fmt.Sprintf("\n[server%d]\n", i+2))
		sb.WriteString(fmt.Sprintf("hostname=%s\n", host))
		sb.WriteString(fmt.Sprintf("port=%d\n", port))
		sb.WriteString("candidate_master=1\n")
	}
	return sb.String()
}

func (e *MHAExecutor) validateMHADeployment(ctx context.Context, config MHAConfig) *TaskResult {
	sshCmd := mhaShellCommand(ctx, config.ManagerHost, config.SSHUser, config.SSHPrivateKey, "masterha_check_ssh --conf=/etc/mha/app1.cnf")
	if out, err := sshCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  70,
			Message:   fmt.Sprintf("MHA SSH check failed: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	replCmd := mhaShellCommand(ctx, config.ManagerHost, config.SSHUser, config.SSHPrivateKey, "masterha_check_repl --conf=/etc/mha/app1.cnf")
	if out, err := replCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  85,
			Message:   fmt.Sprintf("MHA replication check failed: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:   "completed",
		Progress: 100,
		Message: fmt.Sprintf("MHA deployment validated. Manager: %s, Master: %s:%d",
			config.ManagerHost, config.MasterHost, config.MasterPort),
		Timestamp: time.Now(),
	}
}

func mhaShellCommand(ctx context.Context, host string, user string, privateKey string, command string) *exec.Cmd {
	if isLocalHost(host) {
		return exec.CommandContext(ctx, "sh", "-c", command)
	}
	return exec.CommandContext(ctx, "ssh",
		"-i", privateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", user, host),
		command,
	)
}

func isLocalHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil && strings.EqualFold(ip.String(), host) {
			return true
		}
	}
	return false
}

func (e *MHAExecutor) CreateMHAManagerConfig(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := e.parseMHAManagerConfig(req.Config)

	mkdirCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey(),
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
		fmt.Sprintf("mkdir -p %s", config.WorkDir),
	)

	if err := mkdirCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to create MHA work directory: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	configContent := e.generateMHAManagerConfigFile(config)
	configPath := config.ConfigFile
	if configPath == "" {
		configPath = "/etc/mha/app1.cnf"
	}

	writeCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey(),
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
		fmt.Sprintf("echo '%s' > %s", strings.ReplaceAll(configContent, "'", "'\\''"), configPath),
	)

	if err := writeCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Failed to write MHA manager config: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("MHA manager config created at %s", configPath),
		Timestamp: time.Now(),
	}, nil
}

func (e *MHAExecutor) parseMHAManagerConfig(config map[string]interface{}) MHAManagerConfig {
	mc := MHAManagerConfig{
		ConfigFile:   "/etc/mha/app1.cnf",
		ManagerHost:  "localhost",
		MasterPort:   3306,
		VIP:          "192.168.1.100",
		VIPInterface: "eth0",
		ReplUser:     "repl",
		ReplPass:     "repl123",
		SSHUser:      "root",
		WorkDir:      "/var/log/mha/app1",
		PingInterval: 3,
	}

	if v, ok := config["config_file"].(string); ok {
		mc.ConfigFile = v
	}
	if v, ok := config["manager_host"].(string); ok {
		mc.ManagerHost = v
	}
	if v, ok := config["master_host"].(string); ok {
		mc.MasterHost = v
	}
	if v, ok := config["master_port"].(int); ok {
		mc.MasterPort = v
	}
	if v, ok := config["vip"].(string); ok {
		mc.VIP = v
	}
	if v, ok := config["vip_interface"].(string); ok {
		mc.VIPInterface = v
	}
	if v, ok := config["repl_user"].(string); ok {
		mc.ReplUser = v
	}
	if v, ok := config["repl_pass"].(string); ok {
		mc.ReplPass = v
	}
	if v, ok := config["ssh_user"].(string); ok {
		mc.SSHUser = v
	}
	if v, ok := config["work_dir"].(string); ok {
		mc.WorkDir = v
	}
	if v, ok := config["ping_interval"].(int); ok {
		mc.PingInterval = v
	}
	if v, ok := config["candidate_hosts"].([]string); ok {
		mc.CandidateHosts = v
	}

	return mc
}

func (c MHAManagerConfig) SSHPrivateKey() string {
	return "/root/.ssh/id_rsa"
}

func (e *MHAExecutor) generateMHAManagerConfigFile(config MHAManagerConfig) string {
	var sb strings.Builder

	sb.WriteString("[server default]\n")
	sb.WriteString(fmt.Sprintf("manager_workdir=%s\n", config.WorkDir))
	sb.WriteString(fmt.Sprintf("user=%s\n", config.ReplUser))
	sb.WriteString(fmt.Sprintf("password=%s\n", config.ReplPass))
	sb.WriteString(fmt.Sprintf("ssh_user=%s\n", config.SSHUser))
	sb.WriteString(fmt.Sprintf("repl_user=%s\n", config.ReplUser))
	sb.WriteString(fmt.Sprintf("repl_password=%s\n", config.ReplPass))
	sb.WriteString(fmt.Sprintf("ping_interval=%d\n", config.PingInterval))
	sb.WriteString("secondary_check_script=/usr/local/bin/masterha_secondary_check -s remote_host1 -s remote_host2\n")
	sb.WriteString("master_ip_failover_script=/usr/local/bin/master_ip_failover\n")
	sb.WriteString("shutdown_script=/usr/local/bin/power_manager\n")
	sb.WriteString("report_script=/usr/local/bin/send_report\n\n")

	sb.WriteString("[server1]\n")
	sb.WriteString(fmt.Sprintf("hostname=%s\n", config.MasterHost))
	sb.WriteString(fmt.Sprintf("port=%d\n", config.MasterPort))
	sb.WriteString("candidate_master=1\n\n")

	for i, host := range config.CandidateHosts {
		sb.WriteString(fmt.Sprintf("[server%d]\n", i+2))
		sb.WriteString(fmt.Sprintf("hostname=%s\n", host))
		sb.WriteString(fmt.Sprintf("port=%d\n", config.MasterPort))
		sb.WriteString("candidate_master=1\n\n")
	}

	return sb.String()
}

func (e *MHAExecutor) ConfigureMHANode(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := e.parseMHANodeConfig(req.Config)

	installCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.NodeHost),
		"yum install -y mha4mysql-node || apt-get install -y mha4mysql-node || echo 'already installed'",
	)

	if err := installCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to install MHA node on %s: %v", config.NodeHost, err),
			Timestamp: time.Now(),
		}, nil
	}

	createReplUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO '%s'@'%%';",
		config.ReplUser, config.ReplPass, config.ReplUser,
	)

	mysqlCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.NodeHost,
		"-P", fmt.Sprintf("%d", config.NodePort),
		"-u", "root", "-e", createReplUserSQL)

	if err := mysqlCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Failed to create replication user on %s: %v", config.NodeHost, err),
			Timestamp: time.Now(),
		}, nil
	}

	mkdirCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.NodeHost),
		fmt.Sprintf("mkdir -p %s", config.InstallDir),
	)

	if err := mkdirCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  75,
			Message:   fmt.Sprintf("Failed to create install directory: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("MHA node configured on %s:%d", config.NodeHost, config.NodePort),
		Timestamp: time.Now(),
	}, nil
}

func (e *MHAExecutor) parseMHANodeConfig(config map[string]interface{}) MHANodeConfig {
	nc := MHANodeConfig{
		NodeHost:      "localhost",
		NodePort:      3306,
		SSHUser:       "root",
		SSHPrivateKey: "/root/.ssh/id_rsa",
		ReplUser:      "repl",
		ReplPass:      "repl123",
		InstallDir:    "/var/log/mha",
	}

	if v, ok := config["node_host"].(string); ok {
		nc.NodeHost = v
	}
	if v, ok := config["node_port"].(int); ok {
		nc.NodePort = v
	}
	if v, ok := config["ssh_user"].(string); ok {
		nc.SSHUser = v
	}
	if v, ok := config["ssh_private_key"].(string); ok {
		nc.SSHPrivateKey = v
	}
	if v, ok := config["repl_user"].(string); ok {
		nc.ReplUser = v
	}
	if v, ok := config["repl_pass"].(string); ok {
		nc.ReplPass = v
	}
	if v, ok := config["install_dir"].(string); ok {
		nc.InstallDir = v
	}

	return nc
}

func (e *MHAExecutor) DetectMasterFailure(ctx context.Context, req DeployTaskRequest) (*MasterFailureInfo, error) {
	config := parseMHAConfig(req.Config)

	info := &MasterFailureInfo{
		MasterHost:  config.MasterHost,
		MasterPort:  config.MasterPort,
		FailureTime: time.Now(),
	}

	pingCmd := exec.CommandContext(ctx, "ping", "-c", fmt.Sprintf("%d", config.PingRetry), "-W", fmt.Sprintf("%d", config.PingInterval), config.MasterHost)
	if err := pingCmd.Run(); err != nil {
		info.IsReachable = false
	} else {
		info.IsReachable = true
	}

	mysqlCmd := exec.CommandContext(ctx, "mysqladmin",
		"-h", config.MasterHost,
		"-P", fmt.Sprintf("%d", config.MasterPort),
		"-u", config.ReplUser,
		"-p"+config.ReplPass,
		"ping")

	if err := mysqlCmd.Run(); err != nil {
		info.MySQLAlive = false
	} else {
		info.MySQLAlive = true
	}

	if info.MySQLAlive {
		showSlaveCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.MasterHost,
			"-P", fmt.Sprintf("%d", config.MasterPort),
			"-u", config.ReplUser,
			"-p"+config.ReplPass,
			"-e", "SHOW SLAVE STATUS\\G")

		output, err := showSlaveCmd.Output()
		if err == nil && strings.Contains(string(output), "Slave_IO_Running: Yes") {
			info.ReplWorking = true
		} else {
			info.ReplWorking = false
		}
	} else {
		info.ReplWorking = false
	}

	return info, nil
}

func (e *MHAExecutor) ExecuteFailover(ctx context.Context, req DeployTaskRequest) (*FailoverResult, error) {
	config := parseMHAConfig(req.Config)

	failoverCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
		"masterha_master_switch --conf=/etc/mha/app1.cnf --master_state=dead --interactive=0",
	)

	output, err := failoverCmd.CombinedOutput()
	if err != nil {
		return &FailoverResult{
			OldMaster:    fmt.Sprintf("%s:%d", config.MasterHost, config.MasterPort),
			NewMaster:    "unknown",
			FailoverTime: time.Now(),
			VIPSwitched:  false,
			Status:       fmt.Sprintf("failed: %v, output: %s", err, string(output)),
		}, nil
	}

	newMaster := e.extractNewMasterFromOutput(string(output))

	vipResult := e.SwitchVIP(ctx, config, config.MasterHost, newMaster)

	return &FailoverResult{
		OldMaster:    fmt.Sprintf("%s:%d", config.MasterHost, config.MasterPort),
		NewMaster:    newMaster,
		FailoverTime: time.Now(),
		VIPSwitched:  vipResult.Success,
		Status:       "completed",
	}, nil
}

func (e *MHAExecutor) extractNewMasterFromOutput(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "New master is") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				return parts[3]
			}
		}
	}
	return "unknown"
}

func (e *MHAExecutor) SwitchVIP(ctx context.Context, config MHAConfig, oldHost string, newHost string) *VIPSwitchResult {
	result := &VIPSwitchResult{
		VIP:        config.VIP,
		Interface:  config.VIPInterface,
		OldHost:    oldHost,
		NewHost:    newHost,
		SwitchTime: time.Now(),
		Success:    false,
	}

	removeCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, oldHost),
		fmt.Sprintf("ip addr del %s/32 dev %s 2>/dev/null || true", config.VIP, config.VIPInterface),
	)

	_ = removeCmd.Run()

	addCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, newHost),
		fmt.Sprintf("ip addr add %s/32 dev %s", config.VIP, config.VIPInterface),
	)

	if err := addCmd.Run(); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to add VIP on new master: %v", err)
		return result
	}

	arpCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, newHost),
		fmt.Sprintf("arping -c 3 -A %s -I %s", config.VIP, config.VIPInterface),
	)

	if err := arpCmd.Run(); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to send ARP update: %v", err)
		return result
	}

	result.Success = true
	return result
}

func (e *MHAExecutor) ExecuteManualFailover(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMHAConfig(req.Config)

	newMaster := ""
	if v, ok := req.Config["new_master_host"].(string); ok {
		newMaster = v
	}

	if newMaster == "" && len(config.SlaveHosts) > 0 {
		newMaster = config.SlaveHosts[0]
	}

	if newMaster == "" {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "No candidate master found for manual failover",
			Timestamp: time.Now(),
		}, nil
	}

	checkCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, newMaster),
		"mysqladmin ping",
	)

	if err := checkCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  10,
			Message:   fmt.Sprintf("New master %s is not healthy: %v", newMaster, err),
			Timestamp: time.Now(),
		}, nil
	}

	switchCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
		fmt.Sprintf("masterha_master_switch --conf=/etc/mha/app1.cnf --master_state=alive --new_master_host=%s --interactive=0", newMaster),
	)

	output, err := switchCmd.CombinedOutput()
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Manual failover failed: %v, output: %s", err, string(output)),
			Timestamp: time.Now(),
		}, nil
	}

	vipResult := e.SwitchVIP(ctx, config, config.MasterHost, newMaster)
	if !vipResult.Success {
		return &TaskResult{
			Status:    "completed",
			Progress:  90,
			Message:   fmt.Sprintf("Failover completed but VIP switch failed: %s", vipResult.ErrorMessage),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Manual failover completed. New master: %s, VIP switched", newMaster),
		Timestamp: time.Now(),
	}, nil
}

func (e *MHAExecutor) StartMHAManager(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMHAConfig(req.Config)

	startCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
		"nohup masterha_manager --conf=/etc/mha/app1.cnf > /var/log/mha/app1/manager.log 2>&1 &",
	)

	if err := startCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to start MHA manager: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	time.Sleep(2 * time.Second)

	checkCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
		"ps aux | grep masterha_manager | grep -v grep",
	)

	if err := checkCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   "MHA manager process not found after start",
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   "MHA manager started successfully",
		Timestamp: time.Now(),
	}, nil
}

func (e *MHAExecutor) StopMHAManager(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMHAConfig(req.Config)

	stopCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
		"masterha_stop --conf=/etc/mha/app1.cnf",
	)

	if err := stopCmd.Run(); err != nil {
		killCmd := exec.CommandContext(ctx, "ssh",
			"-i", config.SSHPrivateKey,
			"-o", "StrictHostKeyChecking=no",
			fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
			"pkill -f masterha_manager",
		)
		_ = killCmd.Run()
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   "MHA manager stopped",
		Timestamp: time.Now(),
	}, nil
}

func (e *MHAExecutor) GetMHAStatus(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMHAConfig(req.Config)

	statusCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.ManagerHost),
		"masterha_check_status --conf=/etc/mha/app1.cnf",
	)

	output, err := statusCmd.CombinedOutput()
	statusOutput := string(output)

	if err != nil {
		return &TaskResult{
			Status:    "completed",
			Progress:  100,
			Message:   fmt.Sprintf("MHA status check: %s", statusOutput),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("MHA status: %s", statusOutput),
		Timestamp: time.Now(),
	}, nil
}
