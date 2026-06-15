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

	for _, host := range []string{"10.1.81.16", "10.1.81.17"} {
		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("=== %s: SSH fail: %v ===\n", host, err)
			continue
		}

		fmt.Printf("\n=== %s ===\n", host)
		
		// Check GR-related variables
		queries := []string{
			"SHOW VARIABLES LIKE 'group_replication_group_name'",
			"SHOW VARIABLES LIKE 'group_replication_local_address'",
			"SHOW VARIABLES LIKE 'group_replication_group_seeds'",
			"SHOW VARIABLES LIKE 'group_replication_single_primary_mode'",
			"SHOW VARIABLES LIKE 'group_replication_start_on_boot'",
			"SHOW VARIABLES LIKE 'gtid_mode'",
			"SELECT PLUGIN_NAME, PLUGIN_STATUS FROM information_schema.PLUGINS WHERE PLUGIN_NAME='group_replication'",
			"SELECT 1 AS alive",
		}

		for _, q := range queries {
			out := runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "%s" 2>&1`, q), 5)
			if strings.TrimSpace(out) != "" {
				fmt.Printf("  %s => %s\n", q, strings.ReplaceAll(out, "\n", " | "))
			}
		}
		client.Close()
	}
}
