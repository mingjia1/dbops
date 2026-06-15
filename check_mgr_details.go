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

	pass := "Hcfc@DboOps#2024_80"

	// Check host 16 MGR status
	client, err := ssh.Dial("tcp", "10.1.81.16:22", config)
	if err != nil {
		fmt.Printf("SSH 16: %v\n", err)
		return
	}

	fmt.Println("=== Host 16 MGR status ===")
	cmds := []string{
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT @@group_replication_exit_state_action;" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SHOW STATUS LIKE 'group_replication_primary_member';" 2>&1`, pass),
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT @@hostname, @@read_only, @@super_read_only;" 2>&1`, pass),
		`ss -tlnp | grep 3306`,
	}
	for _, cmd := range cmds {
		out := runCmd(client, cmd, 5)
		if out != "" {
			fmt.Println(out)
		}
	}
	client.Close()

	// Check GR error on host 17
	client, err = ssh.Dial("tcp", "10.1.81.17:22", config)
	if err != nil {
		fmt.Printf("SSH 17: %v\n", err)
		return
	}

	fmt.Println("\n=== Host 17 GR connection test ===")
	cmds2 := []string{
		`timeout 3 bash -c 'echo "test" | nc -w 2 10.1.81.16 33061 && echo "16:33061 reachable" || echo "16:33061 UNREACHABLE"' 2>&1 || echo "nc not available"`,
		`cat /data/mysql/3306/error.log | grep -i "timeout\|connection\|error\|unable" | tail -5`,
	}
	for _, cmd := range cmds2 {
		out := runCmd(client, cmd, 10)
		fmt.Println(out)
	}
	client.Close()
}
