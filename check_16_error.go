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
		return "SESSION ERR"
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

	// Check host 16 error log
	client, err := ssh.Dial("tcp", "10.1.81.16:22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}

	fmt.Println("=== Host 16 error log (last 30 lines) ===")
	out := runCmd(client, `tail -30 /data/mysql/3306/error.log 2>/dev/null`, 5)
	fmt.Println(out)

	fmt.Println("\n=== Ports on 16 ===")
	out = runCmd(client, `ss -tlnp | grep 3306`, 5)
	fmt.Println(out)

	// Check GR variables
	fmt.Println("\n=== GR Vars ===")
	for _, v := range []string{
		"group_replication_group_name",
		"group_replication_local_address",
		"group_replication_single_primary_mode",
		"group_replication_bootstrap_group",
		"group_replication_start_on_boot",
	} {
		out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SHOW VARIABLES LIKE '%s';" 2>&1`, pass, v), 5)
		fmt.Printf("  %s\n", out)
	}
	client.Close()
}
