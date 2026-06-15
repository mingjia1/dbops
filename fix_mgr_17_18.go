package main

import (
	"fmt"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

func runCmd(client *ssh.Client, cmd string, timeoutSec int) string {
	s, err := client.NewSession()
	if err != nil {
		return fmt.Sprintf("SESSION ERR: %v", err)
	}
	defer s.Close()
	ch := make(chan string, 1)
	go func() {
		out, _ := s.CombinedOutput(cmd)
		ch <- strings.TrimSpace(string(out))
	}()
	select {
	case res := <-ch:
		return res
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		return "TIMEOUT"
	}
}

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	grName := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	// Fix host 17
	fmt.Println("=== Fix host 17 ===")
	client, err := ssh.Dial("tcp", "10.1.81.17:22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}

	cmds17 := []string{
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL ENFORCE_GTID_CONSISTENCY = ON;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL GTID_MODE = OFF_PERMISSIVE;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL GTID_MODE = ON_PERMISSIVE;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL GTID_MODE = ON;" 2>&1`,
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_group_name = '%s';" 2>&1`, grName),
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_local_address = '10.1.81.17:33062';" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_group_seeds = '10.1.81.16:33061,10.1.81.17:33062,10.1.81.18:33063';" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_ip_allowlist = '10.1.81.16,10.1.81.17,10.1.81.18';" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_single_primary_mode = ON;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_start_on_boot = ON;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_recovery_get_public_key = ON;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "CHANGE MASTER TO MASTER_USER='repl', MASTER_PASSWORD='repl_123' FOR CHANNEL 'group_replication_recovery';" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "START GROUP_REPLICATION;" 2>&1`,
		`sleep 3`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`,
	}

	for _, cmd := range cmds17 {
		out := runCmd(client, cmd, 10)
		if out != "" {
			fmt.Printf("  %s\n", out)
		}
	}
	client.Close()

	fmt.Println("\n=== Now fix host 18 ===")
	client, err = ssh.Dial("tcp", "10.1.81.18:22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}

	pass := "Hcfc@DboOps#2024_80"
	cmds18 := []string{
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_group_name = '%s';" 2>&1`, pass, grName),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_local_address = '10.1.81.18:33063';" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_group_seeds = '10.1.81.16:33061,10.1.81.17:33062,10.1.81.18:33063';" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_ip_allowlist = '10.1.81.16,10.1.81.17,10.1.81.18';" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "CHANGE MASTER TO MASTER_USER='repl', MASTER_PASSWORD='repl_123' FOR CHANNEL 'group_replication_recovery';" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "START GROUP_REPLICATION;" 2>&1`, pass),
		`sleep 5`,
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, pass),
	}

	for _, cmd := range cmds18 {
		out := runCmd(client, cmd, 15)
		if out != "" {
			fmt.Printf("  %s\n", out)
		}
	}
	client.Close()
}
