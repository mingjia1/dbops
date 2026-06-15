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
		return "ERR"
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

func enableGTID(client *ssh.Client, pass string) {
	steps := []string{
		"SET GLOBAL ENFORCE_GTID_CONSISTENCY = ON",
		"SET GLOBAL GTID_MODE = OFF_PERMISSIVE",
		"SET GLOBAL GTID_MODE = ON_PERMISSIVE",
		"SET GLOBAL GTID_MODE = ON",
	}
	for _, s := range steps {
		out := runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "%s" 2>&1`, pass, s), 5)
		fmt.Printf("  %s -> %s\n", s, strings.TrimSpace(out))
	}
}

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	pass := "Hcfc@DboOps#2024_80"
	seeds := "10.1.81.16:33061,10.1.81.17:33062,10.1.81.18:33063"

	// Fix host 17 and 18 GTID first
	for _, h := range []string{"10.1.81.17", "10.1.81.18"} {
		client, err := ssh.Dial("tcp", h+":22", config)
		if err != nil {
			fmt.Printf("%s: SSH fail\n", h)
			continue
		}
		fmt.Printf("\n=== %s: Enable GTID ===\n", h)
		enableGTID(client, pass)
		client.Close()
	}

	// Fix host 16 GTID + configure GR
	client, err := ssh.Dial("tcp", "10.1.81.16:22", config)
	if err != nil {
		fmt.Printf("16: SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("\n=== 16: Enable GTID + GR config ===")
	enableGTID(client, pass)

	// Set GR config
	for _, c := range []string{
		"SET GLOBAL group_replication_group_name = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'",
		"SET GLOBAL group_replication_local_address = '10.1.81.16:33061'",
		fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s'", seeds),
		"SET GLOBAL group_replication_ip_allowlist = '10.1.81.16,10.1.81.17,10.1.81.18'",
		"SET GLOBAL group_replication_single_primary_mode = ON",
		"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF",
		"SET GLOBAL group_replication_start_on_boot = ON",
		"SET GLOBAL group_replication_recovery_get_public_key = ON",
		"CHANGE MASTER TO MASTER_USER='repl', MASTER_PASSWORD='repl_123' FOR CHANNEL 'group_replication_recovery'",
	} {
		out := runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "%s" 2>&1`, pass, c), 5)
		if strings.TrimSpace(out) != "" {
			fmt.Printf("  %s\n", out)
		}
	}

	// Bootstrap host 16
	fmt.Println("\n=== Bootstrap 16 ===")
	out := runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_bootstrap_group=ON; START GROUP_REPLICATION; SET GLOBAL group_replication_bootstrap_group=OFF;" 2>&1`, pass), 15)
	fmt.Println("  START GR:", out)
	time.Sleep(3 * time.Second)

	// Verify
	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, pass), 5)
	fmt.Println("  Status:", out)

	// Check if GR port is listening
	out = runCmd(client, `ss -tlnp | grep 33061`, 5)
	fmt.Println("  GR port 33061:", strings.Contains(out, "LISTEN"))

	// Join host 17
	fmt.Println("\n=== Join 17 ===")
	c17, _ := ssh.Dial("tcp", "10.1.81.17:22", config)
	if c17 != nil {
		for _, c := range []string{
			"SET GLOBAL group_replication_group_name = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'",
			"SET GLOBAL group_replication_local_address = '10.1.81.17:33062'",
			fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s'", seeds),
			"SET GLOBAL group_replication_ip_allowlist = '10.1.81.16,10.1.81.17,10.1.81.18'",
			"SET GLOBAL group_replication_single_primary_mode = ON",
			"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF",
			"SET GLOBAL group_replication_start_on_boot = ON",
			"SET GLOBAL group_replication_recovery_get_public_key = ON",
			"CHANGE MASTER TO MASTER_USER='repl', MASTER_PASSWORD='repl_123' FOR CHANNEL 'group_replication_recovery'",
			"START GROUP_REPLICATION",
		} {
			out := runCmd(c17, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "%s" 2>&1`, pass, c), 15)
			if strings.TrimSpace(out) != "" {
				fmt.Printf("  %s\n", out)
			}
		}
		time.Sleep(3 * time.Second)
		out = runCmd(c17, `mysql -h127.0.0.1 -P3306 -uroot -p'`+pass+`' -e "SELECT member_host, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, 5)
		fmt.Println("  Status:", out)
		c17.Close()
	}

	// Join host 18
	fmt.Println("\n=== Join 18 ===")
	c18, _ := ssh.Dial("tcp", "10.1.81.18:22", config)
	if c18 != nil {
		for _, c := range []string{
			"SET GLOBAL group_replication_group_name = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'",
			"SET GLOBAL group_replication_local_address = '10.1.81.18:33063'",
			fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s'", seeds),
			"SET GLOBAL group_replication_ip_allowlist = '10.1.81.16,10.1.81.17,10.1.81.18'",
			"SET GLOBAL group_replication_single_primary_mode = ON",
			"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF",
			"SET GLOBAL group_replication_start_on_boot = ON",
			"SET GLOBAL group_replication_recovery_get_public_key = ON",
			"CHANGE MASTER TO MASTER_USER='repl', MASTER_PASSWORD='repl_123' FOR CHANNEL 'group_replication_recovery'",
			"START GROUP_REPLICATION",
		} {
			out := runCmd(c18, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "%s" 2>&1`, pass, c), 15)
			if strings.TrimSpace(out) != "" && !strings.Contains(out, "Warning") {
				fmt.Printf("  %s\n", out)
			}
		}
		time.Sleep(3 * time.Second)
		out = runCmd(c18, `mysql -h127.0.0.1 -P3306 -uroot -p'`+pass+`' -e "SELECT member_host, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, 5)
		fmt.Println("  Status:", out)
		c18.Close()
	}

	// Final check from 16
	time.Sleep(2 * time.Second)
	fmt.Println("\n=== Final MGR status ===")
	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, pass), 5)
	fmt.Println(out)
}
