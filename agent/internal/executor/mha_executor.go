package executor

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
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
	SSHPasswords  map[string]string
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
	if v, ok := config["mysql_pass"].(string); ok && mc.MySQLPassword == "" {
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
	if raw, ok := config["ssh_passwords"].(map[string]interface{}); ok {
		mc.SSHPasswords = map[string]string{}
		for host, value := range raw {
			if pass, ok := value.(string); ok {
				mc.SSHPasswords[host] = pass
			}
		}
	} else if raw, ok := config["ssh_passwords"].(map[string]string); ok {
		mc.SSHPasswords = raw
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

	sshResult := e.configureMHAKeyAuth(ctx, config)
	if sshResult.Status == "failed" {
		return sshResult, nil
	}

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

func (e *MHAExecutor) configureMHAKeyAuth(ctx context.Context, config MHAConfig) *TaskResult {
	keyCmd := exec.CommandContext(ctx, "sh", "-c",
		"mkdir -p /root/.ssh && chmod 700 /root/.ssh && "+
			"[ -f /root/.ssh/id_rsa ] || ssh-keygen -q -t rsa -N '' -f /root/.ssh/id_rsa && "+
			"cat /root/.ssh/id_rsa && printf '\\n---PUBLIC---\\n' && cat /root/.ssh/id_rsa.pub")
	out, err := keyCmd.CombinedOutput()
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  5,
			Message:   fmt.Sprintf("Failed to prepare MHA SSH key: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}
	parts := strings.SplitN(string(out), "\n---PUBLIC---\n", 2)
	if len(parts) != 2 {
		return &TaskResult{
			Status:    "failed",
			Progress:  5,
			Message:   "Failed to parse generated MHA SSH key",
			Timestamp: time.Now(),
		}
	}
	privateKey := strings.TrimSpace(parts[0]) + "\n"
	publicKey := strings.TrimSpace(parts[1])
	if publicKey == "" {
		return &TaskResult{
			Status:    "failed",
			Progress:  5,
			Message:   "Generated MHA SSH public key is empty",
			Timestamp: time.Now(),
		}
	}

	allHosts := uniqueStrings(append([]string{config.ManagerHost, config.MasterHost}, config.SlaveHosts...))
	for _, host := range allHosts {
		if isLocalHost(host) {
			cmd := exec.CommandContext(ctx, "sh", "-c", installSSHKeyCommand(privateKey, publicKey))
			if out, err := cmd.CombinedOutput(); err != nil {
				return &TaskResult{
					Status:    "failed",
					Progress:  8,
					Message:   fmt.Sprintf("Failed to authorize local MHA SSH key on %s: %v, output: %s", host, err, strings.TrimSpace(string(out))),
					Timestamp: time.Now(),
				}
			}
			continue
		}
		password := passwordForHost(config.SSHPasswords, host)
		if password == "" {
			return &TaskResult{
				Status:    "failed",
				Progress:  8,
				Message:   fmt.Sprintf("Missing SSH password for MHA host %s", host),
				Timestamp: time.Now(),
			}
		}
		if err := runPasswordSSH(ctx, host, config.SSHUser, password, installSSHKeyCommand(privateKey, publicKey)); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  8,
				Message:   fmt.Sprintf("Failed to authorize MHA SSH key on %s: %v", host, err),
				Timestamp: time.Now(),
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  10,
		Message:   fmt.Sprintf("MHA SSH key authorized on %d hosts", len(allHosts)),
		Timestamp: time.Now(),
	}
}

func installSSHKeyCommand(privateKey, publicKey string) string {
	return "mkdir -p /root/.ssh && chmod 700 /root/.ssh && " +
		"cat > /root/.ssh/id_rsa <<'EOF_KEY'\n" + privateKey + "EOF_KEY\n" +
		"cat > /root/.ssh/id_rsa.pub <<'EOF_PUB'\n" + publicKey + "\nEOF_PUB\n" +
		"touch /root/.ssh/authorized_keys && grep -qxF " + shellQuote(publicKey) + " /root/.ssh/authorized_keys || echo " + shellQuote(publicKey) + " >> /root/.ssh/authorized_keys && " +
		"chmod 600 /root/.ssh/id_rsa /root/.ssh/authorized_keys && chmod 644 /root/.ssh/id_rsa.pub"
}

func runPasswordSSH(ctx context.Context, host, user, password, command string) error {
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(passwordKeyboardInteractive(password)),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(host, "22"), cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = session.Close()
		return ctx.Err()
	}
}

func passwordKeyboardInteractive(password string) ssh.KeyboardInteractiveChallenge {
	return func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range questions {
			answers[i] = password
		}
		return answers, nil
	}
}

func passwordForHost(passwords map[string]string, host string) string {
	if passwords == nil {
		return ""
	}
	if pass := passwords[host]; pass != "" {
		return pass
	}
	for candidate, pass := range passwords {
		if sameIPHost(candidate, host) {
			return pass
		}
	}
	return ""
}

func sameIPHost(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (e *MHAExecutor) installMHANode(ctx context.Context, config MHAConfig) *TaskResult {
	allHosts := append([]string{config.MasterHost}, config.SlaveHosts...)

	for i, host := range allHosts {
		installCmd := mhaShellCommand(ctx, host, config.SSHUser, config.SSHPrivateKey, installMHACommand("node"))
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
		"SET SESSION SQL_LOG_BIN=0; "+
			"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT ALL PRIVILEGES ON *.* TO '%s'@'%%'; FLUSH PRIVILEGES;",
		config.ManagerUser, config.ManagerPass, config.ManagerUser,
	)

	cmd := mysqlExecCommand(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPassword, createUserSQL)
	if out, err := cmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  35,
			Message:   fmt.Sprintf("Failed to create MHA manager user on master %s:%d: %v, output: %s", config.MasterHost, config.MasterPort, err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	for i, host := range config.SlaveHosts {
		port := config.MasterPort
		if i < len(config.SlavePorts) && config.SlavePorts[i] != 0 {
			port = config.SlavePorts[i]
		}
		if repairErr := repairReplicaMHAUser(ctx, host, port, config); repairErr != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  35,
				Message:   fmt.Sprintf("Failed to prepare local MHA manager user on replica %s:%d: %v", host, port, repairErr),
				Timestamp: time.Now(),
			}
		}
		if repairErr := repairReplicaReplicationUser(ctx, host, port, config); repairErr != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  35,
				Message:   fmt.Sprintf("Failed to prepare local MHA replication user on replica %s:%d: %v", host, port, repairErr),
				Timestamp: time.Now(),
			}
		}
		if err := waitForMySQLUser(ctx, host, port, config.ManagerUser, config.ManagerPass); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  35,
				Message:   fmt.Sprintf("MHA manager user was not available on replica %s:%d: %v", host, port, err),
				Timestamp: time.Now(),
			}
		}
	}

	installManagerCmd := mhaShellCommand(ctx, config.ManagerHost, config.SSHUser, config.SSHPrivateKey, installMHACommand("manager"))
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

func repairReplicaMHAUser(ctx context.Context, host string, port int, config MHAConfig) error {
	sql := fmt.Sprintf(
		"SET SESSION SQL_LOG_BIN=0; "+
			"SET GLOBAL super_read_only=OFF; SET GLOBAL read_only=OFF; "+
			"DROP USER IF EXISTS '%s'@'%%'; "+
			"CREATE USER '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT ALL PRIVILEGES ON *.* TO '%s'@'%%'; FLUSH PRIVILEGES; "+
			"SET GLOBAL read_only=ON; SET GLOBAL super_read_only=ON;",
		config.ManagerUser, config.ManagerUser, config.ManagerPass, config.ManagerUser,
	)
	return runReplicaLocalSQL(ctx, host, port, config, sql, true)
}

func repairReplicaReplicationUser(ctx context.Context, host string, port int, config MHAConfig) error {
	sql := fmt.Sprintf(
		"SET SESSION SQL_LOG_BIN=0; "+
			"SET GLOBAL super_read_only=OFF; SET GLOBAL read_only=OFF; "+
			"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO '%s'@'%%'; FLUSH PRIVILEGES; "+
			"SET GLOBAL read_only=ON; SET GLOBAL super_read_only=ON;",
		config.ReplUser, config.ReplPass, config.ReplUser,
	)
	return runReplicaLocalSQL(ctx, host, port, config, sql, true)
}

func runReplicaLocalSQL(ctx context.Context, host string, port int, config MHAConfig, sql string, restoreReadOnly bool) error {
	cmd := mysqlExecCommand(ctx, host, port, config.MySQLUser, config.MySQLPassword, sql)
	if out, err := cmd.CombinedOutput(); err != nil {
		remoteErr := fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
		localCmd := buildReplicaLocalSQLCommand(port, config.MySQLUser, config.MySQLPassword, sql)
		if localOut, localErr := mhaShellCommand(ctx, host, config.SSHUser, config.SSHPrivateKey, localCmd).CombinedOutput(); localErr != nil {
			if restoreReadOnly {
				restoreSQL := "SET GLOBAL read_only=ON; SET GLOBAL super_read_only=ON;"
				restoreCmd := mysqlExecCommand(ctx, host, port, config.MySQLUser, config.MySQLPassword, restoreSQL)
				_, _ = restoreCmd.CombinedOutput()
				restoreLocalCmd := buildReplicaLocalSQLCommand(port, config.MySQLUser, config.MySQLPassword, restoreSQL)
				_, _ = mhaShellCommand(ctx, host, config.SSHUser, config.SSHPrivateKey, restoreLocalCmd).CombinedOutput()
			}
			return fmt.Errorf("%v; local fallback: %v, output: %s", remoteErr, localErr, strings.TrimSpace(string(localOut)))
		}
	}
	return nil
}

func buildReplicaLocalSQLCommand(port int, user, password, sql string) string {
	mysqlBin := shellQuote(resolveBinaryPath("mysql"))
	mysqlUser := shellQuote(defaultString(user, "root"))
	socketPath := shellQuote(filepath.Join("/data/mysql", fmt.Sprintf("%d", port), "mysql.sock"))
	passwordPrefix := ""
	if strings.TrimSpace(password) != "" {
		passwordPrefix = "MYSQL_PWD=" + shellQuote(password) + " "
	}
	sqlArg := shellQuote(sql)
	socketWithPassword := fmt.Sprintf("%s%s -S %s -u%s -e %s", passwordPrefix, mysqlBin, socketPath, mysqlUser, sqlArg)
	socketNoPassword := fmt.Sprintf("%s -S %s -u%s -e %s", mysqlBin, socketPath, mysqlUser, sqlArg)
	tcpWithPassword := fmt.Sprintf("%s%s --protocol=TCP -h127.0.0.1 -P%d -u%s -e %s", passwordPrefix, mysqlBin, port, mysqlUser, sqlArg)
	tcpNoPassword := fmt.Sprintf("%s --protocol=TCP -h127.0.0.1 -P%d -u%s -e %s", mysqlBin, port, mysqlUser, sqlArg)
	return fmt.Sprintf(
		"(%s) || (%s) || (%s) || (%s)",
		socketWithPassword,
		socketNoPassword,
		tcpWithPassword,
		tcpNoPassword,
	)
}

func waitForMySQLUser(ctx context.Context, host string, port int, user, password string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for {
		cmd := mysqlExecCommand(ctx, host, port, user, password, "SELECT 1")
		if out, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else {
			lastErr = fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
		}
		if time.Now().After(deadline) {
			return lastErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func installMHACommand(kind string) string {
	pkg := "mha4mysql-node"
	verify := "save_binary_logs"
	if kind == "manager" {
		pkg = "mha4mysql-manager"
		verify = "masterha_check_ssh masterha_check_repl"
	}
	return fmt.Sprintf(
		"missing=''; for bin in %[2]s; do command -v $bin >/dev/null 2>&1 || missing=\"$missing $bin\"; done; "+
			"if [ -n \"$missing\" ]; then "+
			"echo \"installing %[1]s; missing:$missing\"; "+
			"if command -v yum >/dev/null 2>&1; then yum install -y %[1]s; "+
			"elif command -v apt-get >/dev/null 2>&1; then export DEBIAN_FRONTEND=noninteractive; (dpkg --configure -a || true) && apt-get update && apt-get install -y --fix-missing %[1]s; "+
			"else echo 'no supported package manager found'; exit 1; fi; "+
			"fi; "+
			"missing=''; for bin in %[2]s; do command -v $bin >/dev/null 2>&1 || missing=\"$missing $bin\"; done; "+
			"if [ -n \"$missing\" ]; then echo \"missing MHA command(s) after installing %[1]s:$missing\"; exit 1; fi",
		pkg, verify,
	)
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
	sshCmd := mhaShellCommand(ctx, config.ManagerHost, config.SSHUser, config.SSHPrivateKey, "perl -X $(command -v masterha_check_ssh) --conf=/etc/mha/app1.cnf")
	if out, err := sshCmd.CombinedOutput(); err != nil {
		if !mhaOutputContains(string(out), "All SSH connection tests passed successfully") {
			return &TaskResult{
				Status:    "failed",
				Progress:  70,
				Message:   fmt.Sprintf("MHA SSH check failed: %v, output: %s", err, strings.TrimSpace(string(out))),
				Timestamp: time.Now(),
			}
		}
	}

	for i, host := range config.SlaveHosts {
		port := config.MasterPort
		if i < len(config.SlavePorts) && config.SlavePorts[i] != 0 {
			port = config.SlavePorts[i]
		}
		if err := validateMHAReplica(ctx, host, port, config); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  85,
				Message:   fmt.Sprintf("MHA replication check failed on %s:%d: %v", host, port, err),
				Timestamp: time.Now(),
			}
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

func validateMHAReplica(ctx context.Context, host string, port int, config MHAConfig) error {
	output, err := mysqlExecCommand(ctx, host, port, config.ManagerUser, config.ManagerPass, "SHOW REPLICA STATUS\\G").CombinedOutput()
	if err != nil {
		legacyOutput, legacyErr := mysqlExecCommand(ctx, host, port, config.ManagerUser, config.ManagerPass, "SHOW SLAVE STATUS\\G").CombinedOutput()
		if legacyErr != nil {
			return fmt.Errorf("%v, output: %s; fallback: %v, output: %s", err, strings.TrimSpace(string(output)), legacyErr, strings.TrimSpace(string(legacyOutput)))
		}
		output = legacyOutput
	}

	status := string(output)
	if strings.TrimSpace(status) == "" {
		return fmt.Errorf("replica status is empty")
	}
	if !mhaReplicaStatusContainsRunning(status, "Replica_IO_Running", "Slave_IO_Running") {
		return fmt.Errorf("replica IO thread is not running: %s", strings.TrimSpace(status))
	}
	if !mhaReplicaStatusContainsRunning(status, "Replica_SQL_Running", "Slave_SQL_Running") {
		return fmt.Errorf("replica SQL thread is not running: %s", strings.TrimSpace(status))
	}
	if !strings.Contains(status, "Source_Host: "+config.MasterHost) && !strings.Contains(status, "Master_Host: "+config.MasterHost) {
		return fmt.Errorf("replica source does not match master %s: %s", config.MasterHost, strings.TrimSpace(status))
	}
	return nil
}

func mhaReplicaStatusContainsRunning(status string, modernKey string, legacyKey string) bool {
	return strings.Contains(status, modernKey+": Yes") || strings.Contains(status, legacyKey+": Yes")
}

func mhaOutputContains(output, marker string) bool {
	return strings.Contains(output, marker)
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



