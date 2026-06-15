package main

import (
	"fmt"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

func runCmd(client *ssh.Client, cmd string) string {
	s, err := client.NewSession()
	if err != nil {
		return fmt.Sprintf("SESSION ERR: %v", err)
	}
	defer s.Close()
	out, _ := s.CombinedOutput(cmd)
	return strings.TrimSpace(string(out))
}

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	pass := "Hcfc@DboOps#2024_80"
	grName := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	seeds := "10.1.81.16:33061,10.1.81.17:33062,10.1.81.18:33063"

	client, err := ssh.Dial("tcp", "10.1.81.18:22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("=== Enable GTID and rejoin MGR on host 18 ===")

	cmds := []string{
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL ENFORCE_GTID_CONSISTENCY = ON;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL GTID_MODE = OFF_PERMISSIVE;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL GTID_MODE = ON_PERMISSIVE;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL GTID_MODE = ON;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT @@gtid_mode, @@enforce_gtid_consistency;" 2>&1`, pass),

		// MGR config
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_group_name = '%s';" 2>&1`, pass, grName),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_local_address = '10.1.81.18:33063';" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_group_seeds = '%s';" 2>&1`, pass, seeds),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_ip_allowlist = '10.1.81.16,10.1.81.17,10.1.81.18';" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_single_primary_mode = ON;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_start_on_boot = ON;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_recovery_get_public_key = ON;" 2>&1`, pass),

		// Recovery channel
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "CHANGE MASTER TO MASTER_USER='repl', MASTER_PASSWORD='repl_123' FOR CHANNEL 'group_replication_recovery';" 2>&1`, pass),

		// Start GR
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "START GROUP_REPLICATION;" 2>&1`, pass),
		`sleep 3`,
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, pass),
	}

	for _, cmd := range cmds {
		out := runCmd(client, cmd)
		if strings.TrimSpace(out) != "" {
			fmt.Printf("  %s\n", out)
		}
	}
}
