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

	client, err := ssh.Dial("tcp", "10.1.81.18:22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("=== Check MGR prerequisites on host 18 ===")

	cmds := []string{
		fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SHOW VARIABLES LIKE 'gtid_mode'; SHOW VARIABLES LIKE 'enforce_gtid_consistency'; SHOW VARIABLES LIKE 'group_replication_group_name'; SHOW VARIABLES LIKE 'group_replication_local_address'; SELECT PLUGIN_NAME, PLUGIN_STATUS FROM information_schema.PLUGINS WHERE PLUGIN_NAME='group_replication'; SELECT @@server_uuid; SELECT @@hostname;" 2>&1`, pass),
	}

	for _, cmd := range cmds {
		out := runCmd(client, cmd)
		fmt.Println(out)
	}
}
