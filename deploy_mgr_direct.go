package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Host struct {
	IP   string
	Port int
}

var hosts = []Host{
	{IP: "10.1.81.16", Port: 3306},
	{IP: "10.1.81.17", Port: 3306},
	{IP: "10.1.81.18", Port: 3306},
}

const (
	sshUser     = "root"
	sshPass     = "hcfc!2017"
	replUser    = "repl"
	replPass    = "repl_123"
	mysqlUser   = "root"
	mysqlPass   = ""
	groupName   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	localPort   = 33061
)

func main() {
	config := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.Password(sshPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	groupSeeds := buildGroupSeeds()

	fmt.Println("=== Step 1: Pre-check MySQL on all nodes ===")
	for _, h := range hosts {
		out := runMySQL(config, h, "SELECT 1 AS ok;")
		fmt.Printf("[%s] %s\n", h.IP, out)
	}

	fmt.Println("\n=== Step 2: Bootstrap primary node (16) ===")
	primary := hosts[0]
	primaryResult := bootstrapPrimary(config, primary, groupSeeds[0])
	fmt.Printf("[%s] Bootstrap result:\n%s\n", primary.IP, primaryResult)

	fmt.Println("\n=== Step 3: Join secondary nodes ===")
	for i := 1; i < len(hosts); i++ {
		h := hosts[i]
		result := joinNode(config, h, groupSeeds[i])
		fmt.Printf("[%s] Join result:\n%s\n", h.IP, result)
	}

	fmt.Println("\n=== Step 4: Verify MGR status ===")
	for _, h := range hosts {
		out := runMySQL(config, h, "SELECT * FROM performance_schema.replication_group_members\\G")
		fmt.Printf("[%s]\n%s\n", h.IP, out)
	}
}

func buildGroupSeeds() []string {
	seeds := make([]string, len(hosts))
	for i, h := range hosts {
		seeds[i] = fmt.Sprintf("%s:%d", h.IP, localPort+i)
	}
	return seeds
}

func sshExec(config *ssh.ClientConfig, ip string, cmd string) (string, error) {
	client, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		return "", fmt.Errorf("ssh dial failed: %w", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("session failed: %w", err)
	}
	defer session.Close()
	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

func runMySQL(config *ssh.ClientConfig, h Host, sql string) string {
	passFlag := ""
	if mysqlPass != "" {
		passFlag = fmt.Sprintf(" -p'%s'", mysqlPass)
	}
	cmd := fmt.Sprintf("mysql -h 127.0.0.1 -P %d -u %s%s -Be \"%s\" 2>&1", h.Port, mysqlUser, passFlag, strings.ReplaceAll(sql, "\"", "\\\""))
	out, _ := sshExec(config, h.IP, cmd)
	return out
}

func bootstrapPrimary(config *ssh.ClientConfig, h Host, seed string) string {
	primaryActions := []string{
		"STOP GROUP_REPLICATION;",
		"SET GLOBAL super_read_only = OFF;",
		"SET GLOBAL read_only = OFF;",
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';", replUser, replPass),
		fmt.Sprintf("ALTER USER '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';", replUser, replPass),
		fmt.Sprintf("GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%';", replUser),
		"INSTALL PLUGIN group_replication SONAME 'group_replication.so';",
		fmt.Sprintf("SET GLOBAL group_replication_group_name = '%s';", groupName),
		"SET GLOBAL group_replication_start_on_boot = OFF;",
		"SET GLOBAL group_replication_bootstrap_group = ON;",
		fmt.Sprintf("SET GLOBAL group_replication_local_address = '%s';", seed),
		fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s';", seed),
		"SET GLOBAL group_replication_ip_whitelist = '10.1.81.16,10.1.81.17,10.1.81.18';",
		"SET GLOBAL group_replication_single_primary_mode = ON;",
		"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF;",
		"SET GLOBAL group_replication_components_stop_timeout = 300;",
		fmt.Sprintf("CHANGE MASTER TO MASTER_USER='%s', MASTER_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery';", replUser, replPass),
		"START GROUP_REPLICATION;",
		"SELECT * FROM performance_schema.replication_group_members;",
	}
	var sb strings.Builder
	for _, action := range primaryActions {
		sb.WriteString(action + "\n")
	}
	return runMySQL(config, h, sb.String())
}

func joinNode(config *ssh.ClientConfig, h Host, seed string) string {
	allSeeds := strings.Join(buildGroupSeeds(), ",")
	joinActions := []string{
		"STOP GROUP_REPLICATION;",
		"SET GLOBAL super_read_only = OFF;",
		"SET GLOBAL read_only = OFF;",
		"INSTALL PLUGIN group_replication SONAME 'group_replication.so';",
		fmt.Sprintf("SET GLOBAL group_replication_group_name = '%s';", groupName),
		"SET GLOBAL group_replication_start_on_boot = OFF;",
		"SET GLOBAL group_replication_bootstrap_group = OFF;",
		fmt.Sprintf("SET GLOBAL group_replication_local_address = '%s';", seed),
		fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s';", allSeeds),
		"SET GLOBAL group_replication_ip_whitelist = '10.1.81.16,10.1.81.17,10.1.81.18';",
		"SET GLOBAL group_replication_single_primary_mode = ON;",
		"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF;",
		"SET GLOBAL group_replication_components_stop_timeout = 300;",
		fmt.Sprintf("CHANGE MASTER TO MASTER_USER='%s', MASTER_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery';", replUser, replPass),
		"START GROUP_REPLICATION;",
		"SELECT * FROM performance_schema.replication_group_members;",
	}
	var sb strings.Builder
	for _, action := range joinActions {
		sb.WriteString(action + "\n")
	}
	return runMySQL(config, h, sb.String())
}
