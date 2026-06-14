package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Node struct {
	IP        string
	MySQLPort int
	GRPort    int
	Idx       int
}

var (
	sshUser   = "root"
	sshPass   = "hcfc!2017"
	replUser  = "repl"
	replPass  = "repl_123"
	mysqlUser = "root"
	mysqlPass = ""
	grName    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	nodes     = []Node{
		{IP: "10.1.81.16", MySQLPort: 3306, GRPort: 33061, Idx: 0},
		{IP: "10.1.81.17", MySQLPort: 3306, GRPort: 33062, Idx: 1},
		{IP: "10.1.81.18", MySQLPort: 3306, GRPort: 33063, Idx: 2},
	}
)

func newConfig() *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.Password(sshPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
}

func runOn(ip, cmd string) string {
	client, err := ssh.Dial("tcp", ip+":22", newConfig())
	if err != nil {
		return fmt.Sprintf("SSH ERR: %v", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return fmt.Sprintf("SESS ERR: %v", err)
	}
	defer session.Close()
	out, _ := session.CombinedOutput(cmd)
	return string(out)
}

func runMySQL(n Node, sql string) string {
	passFlag := ""
	if mysqlPass != "" {
		passFlag = fmt.Sprintf(" -p'%s'", mysqlPass)
	}
	cmd := fmt.Sprintf("mysql -h127.0.0.1 -P%d -u%s%s -e \"%s\" 2>&1",
		n.MySQLPort, mysqlUser, passFlag, strings.ReplaceAll(sql, "\"", "\\\""))
	return runOn(n.IP, cmd)
}

func main() {
	seeds := make([]string, len(nodes))
	for i, n := range nodes {
		seeds[i] = fmt.Sprintf("%s:%d", n.IP, n.GRPort)
	}
	allSeeds := strings.Join(seeds, ",")

	fmt.Println("=== Pre-check MySQL on all nodes ===")
	for _, n := range nodes {
		out := runMySQL(n, "SELECT CONCAT(@@version, ' ', @@hostname)")
		fmt.Printf("[%s] %s", n.IP, out)
	}

	fmt.Println("\n=== Step 1: STOP GROUP_REPLICATION on all nodes if running ===")
	for _, n := range nodes {
		out := runMySQL(n, "STOP GROUP_REPLICATION")
		fmt.Printf("[%s] %s", n.IP, strings.TrimSpace(out))
	}

	fmt.Println("\n=== Step 2: RESET MASTER on all nodes (clears GTID_EXECUTED) ===")
	for _, n := range nodes {
		if o := runMySQL(n, "RESET MASTER"); strings.TrimSpace(o) != "" {
			fmt.Printf("[%s] RESET MASTER: %s\n", n.IP, strings.TrimSpace(o))
		} else {
			fmt.Printf("[%s] RESET MASTER OK\n", n.IP)
		}
	}

	fmt.Println("\n=== Step 3: Set MGR sys vars on all nodes ===")
	for _, n := range nodes {
		sql := fmt.Sprintf("SET GLOBAL group_replication_group_name = '%s';", grName)
		sql += " SET GLOBAL group_replication_start_on_boot = OFF;"
		sql += fmt.Sprintf(" SET GLOBAL group_replication_local_address = '%s';", seeds[n.Idx])
		sql += fmt.Sprintf(" SET GLOBAL group_replication_group_seeds = '%s';", allSeeds)
		sql += " SET GLOBAL group_replication_ip_allowlist = '10.1.81.16,10.1.81.17,10.1.81.18';"
		sql += " SET GLOBAL group_replication_single_primary_mode = ON;"
		sql += " SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF;"
		sql += " SET GLOBAL group_replication_components_stop_timeout = 300;"
		sql += " SET GLOBAL group_replication_flow_control_mode = DISABLED;"
		sql += " SET GLOBAL group_replication_recovery_get_public_key = ON;"
		sql += " SET GLOBAL group_replication_exit_state_action = ABORT_SERVER;"
		out := runMySQL(n, sql)
		if strings.TrimSpace(out) != "" {
			fmt.Printf("[%s] config ERR: %s\n", n.IP, strings.TrimSpace(out))
		} else {
			fmt.Printf("[%s] config OK\n", n.IP)
		}
	}

	fmt.Println("\n=== Step 4: Create replication user on all nodes ===")
	for _, n := range nodes {
		sql := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';", replUser, replPass)
		sql += fmt.Sprintf(" GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%';", replUser)
		sql += fmt.Sprintf(" GRANT BACKUP_ADMIN ON *.* TO '%s'@'%%';", replUser)
		sql += fmt.Sprintf(" GRANT GROUP_REPLICATION_STREAM ON *.* TO '%s'@'%%';", replUser)
		sql += " FLUSH PRIVILEGES;"
		if o := runMySQL(n, sql); strings.TrimSpace(o) != "" {
			fmt.Printf("[%s] user: %s\n", n.IP, strings.TrimSpace(o))
		} else {
			fmt.Printf("[%s] user OK\n", n.IP)
		}
	}

	fmt.Println("\n=== Step 5: RESET MASTER again after user creation ===")
	for _, n := range nodes {
		if o := runMySQL(n, "RESET MASTER"); strings.TrimSpace(o) != "" {
			fmt.Printf("[%s] RESET MASTER: %s\n", n.IP, strings.TrimSpace(o))
		} else {
			fmt.Printf("[%s] RESET MASTER OK\n", n.IP)
		}
	}

	fmt.Println("\n=== Step 6: Bootstrap primary (16) ===")
	primary := nodes[0]
	out := runMySQL(primary, "SELECT @@gtid_mode, @@log_bin, @@server_id, @@gtid_executed")
	fmt.Printf("[%s] pre-check: %s\n", primary.IP, strings.TrimSpace(out))

	out = runMySQL(primary, "RESET SLAVE ALL FOR CHANNEL 'group_replication_recovery'")
	fmt.Printf("[%s] reset slave: %s\n", primary.IP, strings.TrimSpace(out))

	out = runMySQL(primary, fmt.Sprintf("CHANGE MASTER TO MASTER_USER='%s', MASTER_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery'", replUser, replPass))
	fmt.Printf("[%s] change master: %s\n", primary.IP, strings.TrimSpace(out))

	out = runMySQL(primary, "SET GLOBAL group_replication_bootstrap_group = ON")
	fmt.Printf("[%s] bootstrap=ON: %s\n", primary.IP, strings.TrimSpace(out))

	out = runMySQL(primary, "START GROUP_REPLICATION")
	fmt.Printf("[%s] start GR: %s\n", primary.IP, strings.TrimSpace(out))

	out = runMySQL(primary, "SET GLOBAL group_replication_bootstrap_group = OFF")
	fmt.Printf("[%s] bootstrap=OFF: %s\n", primary.IP, strings.TrimSpace(out))

	out = runMySQL(primary, "SELECT * FROM performance_schema.replication_group_members")
	fmt.Printf("[%s] status:\n%s\n", primary.IP, out)

	fmt.Println("\n=== Step 7: Join secondaries ===")
	for i := 1; i < len(nodes); i++ {
		n := nodes[i]
		time.Sleep(3 * time.Second)
		fmt.Printf("[%s] joining...\n", n.IP)

		out = runMySQL(n, "SELECT @@gtid_executed")
		fmt.Printf("[%s] gtid_executed: %s\n", n.IP, strings.TrimSpace(out))

		out = runMySQL(n, "RESET SLAVE ALL FOR CHANNEL 'group_replication_recovery'")
		fmt.Printf("[%s] reset slave: %s\n", n.IP, strings.TrimSpace(out))

		out = runMySQL(n, fmt.Sprintf("CHANGE MASTER TO MASTER_USER='%s', MASTER_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery'", replUser, replPass))
		fmt.Printf("[%s] change master: %s\n", n.IP, strings.TrimSpace(out))

		out = runMySQL(n, "START GROUP_REPLICATION")
		fmt.Printf("[%s] start GR:\n%s\n", n.IP, out)
	}

	time.Sleep(5 * time.Second)
	fmt.Println("\n=== Step 8: Verify MGR cluster status ===")
	for _, n := range nodes {
		out := runMySQL(n, "SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members")
		fmt.Printf("[%s]\n%s\n", n.IP, out)
	}

	fmt.Println("\n=== Step 9: Check GTID executed on all nodes ===")
	for _, n := range nodes {
		out := runMySQL(n, "SELECT @@gtid_executed")
		fmt.Printf("[%s] gtid_executed: %s\n", n.IP, strings.TrimSpace(out))
	}
}
