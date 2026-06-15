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

	// Check host 16 via repl user
	client, err := ssh.Dial("tcp", "10.1.81.16:22", config)
	if err != nil {
		fmt.Printf("SSH 16: %v\n", err)
		return
	}

	fmt.Println("=== Host 16 via repl ===")
	cmds := []string{
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SELECT @@group_replication_exit_state_action, @@group_replication_force_members;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SELECT @@hostname, @@read_only, @@super_read_only, @@gtid_mode;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SELECT @@group_replication_single_primary_mode;" 2>&1`,
	}

	for _, cmd := range cmds {
		out := runCmd(client, cmd, 5)
		fmt.Println(out)
	}
	client.Close()

	// Try empty password on host 16
	client, err = ssh.Dial("tcp", "10.1.81.16:22", config)
	if err != nil {
		fmt.Printf("SSH 16: %v\n", err)
		return
	}
	fmt.Println("\n=== Host 16 test: empty password ===")
	out := runCmd(client, `mysql -h127.0.0.1 -P3306 -uroot -e "SELECT 'EMPTY PWD OK' AS s;" 2>&1`, 5)
	fmt.Println(out)
	client.Close()

	// Check GTID_EXECUTED on all hosts
	for _, h := range []string{"10.1.81.16", "10.1.81.17", "10.1.81.18"} {
		client, err := ssh.Dial("tcp", h+":22", config)
		if err != nil {
			fmt.Printf("%s SSH fail: %v\n", h, err)
			continue
		}
		fmt.Printf("\n=== %s GTID ===\n", h)
		out := runCmd(client, `mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SELECT @@gtid_executed, @@gtid_purged;" 2>&1`, 5)
		fmt.Println(out)
		client.Close()
	}
}
