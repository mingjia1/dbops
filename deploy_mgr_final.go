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
	ID        int
}

var (
	sshUser  = "root"
	sshPass  = "hcfc!2017"
	replUser = "repl"
	replPass = "repl_123"
	grName   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	nodes = []Node{
		{IP: "10.1.81.16", MySQLPort: 3306, GRPort: 33061, ID: 1},
		{IP: "10.1.81.17", MySQLPort: 3306, GRPort: 33062, ID: 2},
		{IP: "10.1.81.18", MySQLPort: 3306, GRPort: 33063, ID: 3},
	}
)

func buildSeeds() []string {
	s := make([]string, len(nodes))
	for i, n := range nodes {
		s[i] = fmt.Sprintf("%s:%d", n.IP, n.GRPort)
	}
	return s
}

func sshExec(ip, cmd string) (string, error) {
	config := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.Password(sshPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		return "", err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

func runSQL(ip string, port int, sql string) string {
	escapedSQL := strings.ReplaceAll(sql, "'", "'\\''")
	cmd := fmt.Sprintf("mysql -h127.0.0.1 -P%d -uroot -e '%s' 2>&1", port, escapedSQL)
	out, _ := sshExec(ip, cmd)
	return out
}

func main() {
	seeds := buildSeeds()
	allSeeds := strings.Join(seeds, ",")

	fmt.Println("=== Pre-check connectivity ===")
	for _, n := range nodes {
		out := runSQL(n.IP, n.MySQLPort, "SELECT CONCAT(@@version, ' ', @@hostname) AS info")
		fmt.Printf("[%s] %s\n", n.IP, strings.TrimSpace(out))
	}

	fmt.Println("\n=== Step 1: Install group_replication plugin on all nodes ===")
	for _, n := range nodes {
		out := runSQL(n.IP, n.MySQLPort, "INSTALL PLUGIN group_replication SONAME 'group_replication.so'")
		fmt.Printf("[%s] install plugin:\n%s\n", n.IP, out)
		out = runSQL(n.IP, n.MySQLPort, "SELECT PLUGIN_NAME, PLUGIN_STATUS FROM information_schema.PLUGINS WHERE PLUGIN_NAME = 'group_replication'")
		fmt.Printf("[%s] verify:\n%s\n", n.IP, out)
	}

	fmt.Println("\n=== Step 2: Configure MGR variables on all nodes ===")
	for _, n := range nodes {
		cmds := []string{
			fmt.Sprintf("SET GLOBAL group_replication_group_name = '%s'", grName),
			"SET GLOBAL group_replication_start_on_boot = OFF",
			fmt.Sprintf("SET GLOBAL group_replication_local_address = '%s'", seeds[n.ID-1]),
			fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s'", allSeeds),
			"SET GLOBAL group_replication_ip_whitelist = '10.1.81.16,10.1.81.17,10.1.81.18'",
			"SET GLOBAL group_replication_single_primary_mode = ON",
			"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF",
			"SET GLOBAL group_replication_components_stop_timeout = 300",
			"SET GLOBAL group_replication_auto_increment_increment = 1",
			"SET GLOBAL group_replication_exit_state_action = ABORT_SERVER",
			"SET GLOBAL group_replication_flow_control_mode = DISABLED",
			"SET GLOBAL group_replication_recovery_get_public_key = ON",
		}
		for _, cmd := range cmds {
			out := runSQL(n.IP, n.MySQLPort, cmd)
			if out != "" && !strings.Contains(out, "OK") && !strings.Contains(out, "0 rows") {
				fmt.Printf("[%s] %s => %s", n.IP, cmd, out)
			}
		}
		fmt.Printf("[%s] config done\n", n.IP)
	}

	fmt.Println("\n=== Step 3: Create replication user on all nodes ===")
	for _, n := range nodes {
		sql := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s'; ALTER USER '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s'; GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%'; SELECT User, Host FROM mysql.user WHERE User='%s'",
			replUser, replPass, replUser, replPass, replUser, replUser)
		out := runSQL(n.IP, n.MySQLPort, sql)
		fmt.Printf("[%s] user:\n%s\n", n.IP, out)
	}

	fmt.Println("\n=== Step 4: Bootstrap primary (16) ===")
	primary := nodes[0]
	cmds := []string{
		fmt.Sprintf("CHANGE MASTER TO MASTER_USER='%s', MASTER_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery'", replUser, replPass),
		"SET GLOBAL group_replication_bootstrap_group = ON",
		"START GROUP_REPLICATION",
	}
	for _, cmd := range cmds {
		out := runSQL(primary.IP, primary.MySQLPort, cmd)
		fmt.Printf("[%s] %s\n=> %s\n", primary.IP, cmd, out)
	}
	out := runSQL(primary.IP, primary.MySQLPort, "SET GLOBAL group_replication_bootstrap_group = OFF")
	fmt.Printf("[%s] bootstrap_group=OFF => %s\n", primary.IP, out)
	out = runSQL(primary.IP, primary.MySQLPort, "SELECT * FROM performance_schema.replication_group_members")
	fmt.Printf("[%s] status:\n%s\n", primary.IP, out)

	fmt.Println("\n=== Step 5: Join secondaries (17, 18) ===")
	for i := 1; i < len(nodes); i++ {
		n := nodes[i]
		time.Sleep(2 * time.Second)
		cmd := fmt.Sprintf("CHANGE MASTER TO MASTER_USER='%s', MASTER_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery'", replUser, replPass)
		out := runSQL(n.IP, n.MySQLPort, cmd)
		fmt.Printf("[%s] change master:\n%s\n", n.IP, out)
		time.Sleep(1 * time.Second)
		out = runSQL(n.IP, n.MySQLPort, "START GROUP_REPLICATION")
		fmt.Printf("[%s] start GR:\n%s\n", n.IP, out)
	}

	time.Sleep(3 * time.Second)
	fmt.Println("\n=== Step 6: Verify MGR status ===")
	for _, n := range nodes {
		out := runSQL(n.IP, n.MySQLPort, "SELECT * FROM performance_schema.replication_group_members")
		fmt.Printf("[%s]\n%s\n", n.IP, out)
	}

	fmt.Println("\n=== Step 7: Update platform deployment record ===")
	out2 := runSQL(nodes[0].IP, nodes[0].MySQLPort, "SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members")
	fmt.Printf("[PRIMARY] Final status:\n%s\n", out2)
}
