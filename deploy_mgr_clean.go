package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/ssh"
)

func sshRun(host, cmd string) {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		fmt.Printf("[%s] SSH ERROR: %v\n", host, err)
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		fmt.Printf("[%s] Session ERROR: %v\n", host, err)
		return
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	fmt.Printf("[%s] Output:\n%s\n", host, string(output))
	if err != nil {
		fmt.Printf("[%s] RC: %v\n", host, err)
	}
}

func main() {
	hosts := []string{"10.1.81.16", "10.1.81.17", "10.1.81.18"}

	// Step 1: Stop MySQL on all hosts
	fmt.Println("=== Stopping MySQL on all hosts ===")
	for _, h := range hosts {
		sshRun(h, "pkill -9 mysqld 2>/dev/null; sleep 2; rm -rf /data/mysql/* 2>/dev/null; echo 'cleaned'")
	}
	time.Sleep(3 * time.Second)

	// Step 2: Init and start MySQL on all hosts
	fmt.Println("\n=== Init and start MySQL on all hosts ===")
	for _, h := range hosts {
		sshRun(h, "mkdir -p /data/mysql/3306; chown -R mysql:mysql /data/mysql 2>/dev/null; "+
			"mysqld --no-defaults --initialize-insecure --datadir=/data/mysql/3306 --user=mysql 2>&1 | tail -3; "+
			"chown -R mysql:mysql /data/mysql/3306; echo 'init done'")
	}
	time.Sleep(5 * time.Second)

	for _, h := range hosts {
		sshRun(h, "/usr/sbin/mysqld --no-defaults --daemonize "+
			"--datadir=/data/mysql/3306 --port=3306 --server-id=3306 "+
			"--log-bin=mysql-bin --binlog-format=ROW --binlog-checksum=NONE "+
			"--gtid-mode=ON --enforce-gtid-consistency=ON --log-slave-updates=ON "+
			"--bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock "+
			"--pid-file=/data/mysql/3306/mysql.pid --log-error=/data/mysql/3306/error.log "+
			"--transaction-write-set-extraction=XXHASH64 --user=mysql 2>&1; echo 'start done'")
	}
	time.Sleep(10 * time.Second)

	// Step 3: Verify MySQL running
	fmt.Println("\n=== Verify MySQL on all hosts ===")
	for _, h := range hosts {
		sshRun(h, "mysqladmin -h127.0.0.1 -P3306 -uroot ping 2>&1")
	}

	// Step 4: Check mysql client PATH on agent
	fmt.Println("\n=== Check mysql PATH via agent ===")
	for _, h := range hosts {
		sshRun(h, "cat /proc/$(cat /opt/dbops-agent/agent.pid 2>/dev/null || pidof agent 2>/dev/null)/environ 2>/dev/null | tr '\\0' '\\n' | grep PATH || echo 'no path info'; which mysql 2>/dev/null || echo 'mysql not in PATH'")
	}

	// Step 5: Create a simple test connection
	fmt.Println("\n=== Test mysql connection with root ===")
	for _, h := range hosts {
		sshRun(h, "mysql -h127.0.0.1 -P3306 -uroot -e 'SELECT VERSION();' 2>&1")
	}

	// Step 6: Deploy MGR via API
	fmt.Println("\n=== Deploy MGR via Platform API ===")
	loginBody := `{"username":"codexadmin131813","password":"admin123"}`
	loginResp, err := http.Post("http://localhost:8080/api/v1/auth/login", "application/json", bytes.NewBufferString(loginBody))
	if err != nil {
		fmt.Printf("LOGIN FAILED: %v\n", err)
		return
	}
	defer loginResp.Body.Close()
	var loginResult struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult.Data.Token
	fmt.Printf("Token: %s...\n", token[:30])

	// Get hosts
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/v1/hosts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("HOSTS FAILED: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var hostsResult struct {
		Data []struct {
			ID      string `json:"id"`
			Address string `json:"address"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&hostsResult)

	var host16, host17, host18 string
	for _, h := range hostsResult.Data {
		switch h.Address {
		case "10.1.81.16":
			host16 = h.ID
		case "10.1.81.17":
			host17 = h.ID
		case "10.1.81.18":
			host18 = h.ID
		}
	}
	fmt.Printf("16: %s, 17: %s, 18: %s\n", host16, host17, host18)

	deployReq := map[string]interface{}{
		"cluster_id":       "mgr-3node",
		"name":             "MGR 3-Node",
		"primary_host_id":  host16,
		"primary_port":     3306,
		"replica_host_ids": []string{host17, host18},
		"replica_port":     3306,
		"mysql_user":       "root",
		"mysql_password":   "",
	}
	deployBody, _ := json.Marshal(deployReq)

	req2, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/deployments/mgr", bytes.NewBuffer(deployBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		fmt.Printf("DEPLOY FAILED: %v\n", err)
		return
	}
	defer resp2.Body.Close()

	respBody, _ := io.ReadAll(resp2.Body)
	fmt.Printf("Deploy response (%d):\n%s\n", resp2.StatusCode, string(respBody))
}
