package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type HostTask struct {
	Host string
	Cmds []string
}

func sshRun(host string, cmds []string) error {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return fmt.Errorf("connect %s: %w", host, err)
	}
	defer client.Close()

	cmd := strings.Join(cmds, "; ")
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session %s: %w", host, err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	fmt.Printf("[%s] Output:\n%s\n", host, string(out))
	if err != nil {
		return fmt.Errorf("run on %s: %w", host, err)
	}
	return nil
}

func checkMySQLOnHost(host string) (bool, error) {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return false, fmt.Errorf("connect %s: %w", host, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return false, err
	}
	defer session.Close()

	out, err := session.CombinedOutput("which mysqld 2>/dev/null && mysqld --version 2>/dev/null || echo 'NOT_FOUND'")
	if err != nil {
		return false, err
	}
	return !strings.Contains(string(out), "NOT_FOUND"), nil
}

func main() {
	hosts := []string{"10.1.81.16", "10.1.81.17", "10.1.81.18"}

	fmt.Println("=== Step 1: Check MySQL installation on all hosts ===")
	var wg sync.WaitGroup
	type checkResult struct {
		host  string
		found bool
		err   error
	}
	results := make([]checkResult, len(hosts))
	for i, h := range hosts {
		wg.Add(1)
		go func(idx int, host string) {
			defer wg.Done()
			found, err := checkMySQLOnHost(host)
			results[idx] = checkResult{host, found, err}
		}(i, h)
	}
	wg.Wait()

	needReinstall := false
	for _, r := range results {
		if r.err != nil {
			fmt.Printf("[%s] ERROR: %v\n", r.host, r.err)
		} else if r.found {
			fmt.Printf("[%s] MySQL is installed\n", r.host)
		} else {
			fmt.Printf("[%s] MySQL NOT found - needs reinstall\n", r.host)
			needReinstall = true
		}
	}

	if needReinstall {
		fmt.Println("\n=== Step 2: Reinstall MySQL on 18 ===")
		// On 18, reinstall percona xtradb cluster 5.7
		err := sshRun("10.1.81.18", []string{
			"apt-get update -qq 2>/dev/null || true",
			"DEBIAN_FRONTEND=noninteractive apt-get install -y -qq percona-xtradb-cluster-server-5.7 2>/dev/null || " +
				"DEBIAN_FRONTEND=noninteractive apt-get install -y -qq percona-xtradb-cluster-57 2>/dev/null || " +
				"DEBIAN_FRONTEND=noninteractive apt-get install -y -qq percona-server-server-5.7 2>/dev/null || true",
			"which mysqld 2>/dev/null || ln -sf /usr/sbin/mysqld /usr/bin/mysqld 2>/dev/null; echo 'check:'; which mysqld || echo 'STILL NOT FOUND'",
		})
		if err != nil {
			fmt.Printf("Reinstall on 18 failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("\n=== Step 3: Initialize data directories and start MySQL ===")
	for _, host := range hosts {
		h := host
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Printf("[%s] Initializing datadir and starting MySQL...\n", h)
			script := "mkdir -p /data/mysql/3306; chown -R mysql:mysql /data/mysql; " +
				"if [ ! -f /data/mysql/3306/ibdata1 ]; then " +
				"mysqld --no-defaults --initialize-insecure --datadir=/data/mysql/3306 --user=mysql 2>/dev/null || true; " +
				"echo 'Datadir initialized'; " +
				"else echo 'Datadir already initialized'; fi; " +
				"chown -R mysql:mysql /data/mysql/3306; " +
				"pkill -9 mysqld 2>/dev/null || true; sleep 2; " +
				"/usr/sbin/mysqld --no-defaults --daemonize " +
				"--datadir=/data/mysql/3306 --port=3306 --server-id=3306 " +
				"--log-bin=mysql-bin --binlog-format=ROW --binlog-checksum=NONE " +
				"--gtid-mode=ON --enforce-gtid-consistency=ON --log-slave-updates=ON " +
				"--bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock " +
				"--pid-file=/data/mysql/3306/mysql.pid " +
				"--log-error=/data/mysql/3306/error.log " +
				"--transaction-write-set-extraction=XXHASH64 " +
				"--user=mysql 2>/dev/null || true; sleep 3; " +
				"if [ -f /data/mysql/3306/mysql.pid ]; then " +
				"pid=$(cat /data/mysql/3306/mysql.pid 2>/dev/null); " +
				"if [ -n \"$pid\" ] && kill -0 $pid 2>/dev/null; then " +
				"echo 'MySQL started successfully (PID: '$pid')'; " +
				"else echo 'MySQL PID file exists but process not running'; " +
				"cat /data/mysql/3306/error.log 2>/dev/null | tail -20; fi; " +
				"else echo 'MySQL PID file not found'; " +
				"cat /data/mysql/3306/error.log 2>/dev/null | tail -20; fi"
			err := sshRun(h, []string{script})
			if err != nil {
				fmt.Printf("[%s] ERROR: %v\n", h, err)
			}
		}()
	}
	wg.Wait()

	// Wait for MySQL to be ready
	fmt.Println("\n=== Waiting 15 seconds for MySQL to stabilize ===")
	time.Sleep(15 * time.Second)

	fmt.Println("\n=== Step 4: Check MySQL connectivity on each host ===")
	for _, host := range hosts {
		err := sshRun(host, []string{
			"mysqladmin -h127.0.0.1 -P3306 -uroot ping 2>&1 || mysql -h127.0.0.1 -P3306 -uroot -e 'SELECT 1' 2>&1 || echo 'MySQL not reachable'",
		})
		if err != nil {
			fmt.Printf("[%s] Connectivity check failed: %v\n", host, err)
		}
	}

	fmt.Println("\n=== Step 5: Deploy 3-node MGR via Platform API ===")
	// Login and get token
	loginBody := `{"username":"codexadmin131813","password":"admin123"}`
	loginResp, err := http.Post("http://localhost:8080/api/v1/auth/login", "application/json", bytes.NewBufferString(loginBody))
	if err != nil {
		fmt.Printf("LOGIN FAILED: %v\n", err)
		os.Exit(1)
	}
	defer loginResp.Body.Close()

	var loginResult struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult.Data.Token
	fmt.Printf("Token obtained: %s...\n", token[:30])

	// Get hosts
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/v1/hosts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("HOST LIST FAILED: %v\n", err)
		os.Exit(1)
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
	fmt.Printf("Host IDs - 16: %s, 17: %s, 18: %s\n", host16, host17, host18)

	// Deploy MGR via backend API
	deployReq := map[string]interface{}{
		"cluster_id":       "mgr-3node-prod-v2",
		"name":             "MGR 3-Node Cluster v2",
		"primary_host_id":  host16,
		"primary_port":     3306,
		"replica_host_ids": []string{host17, host18},
		"replica_port":     3306,
		"mysql_user":       "root",
		"mysql_password":   "root",
	}

	deployBody, _ := json.Marshal(deployReq)
	fmt.Printf("Deploy request: %s\n", string(deployBody))

	req2, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/deployments/mgr", bytes.NewBuffer(deployBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		fmt.Printf("DEPLOY FAILED: %v\n", err)
		os.Exit(1)
	}
	defer resp2.Body.Close()

	respBody, _ := io.ReadAll(resp2.Body)
	fmt.Printf("Deploy response (%d):\n%s\n", resp2.StatusCode, string(respBody))

	fmt.Println("\n=== Done ===")
}
