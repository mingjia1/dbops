package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func main() {
	// 直接测试16 Agent的deploy API (MGR)
	agentURL := "http://10.1.81.16:9090"

	deployReq := map[string]interface{}{
		"task_id":     "direct-mgr-test",
		"instance_id": "",
		"config": map[string]interface{}{
			"deploy_mode":    "mgr",
			"group_name":     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			"group_seeds":    []string{},
			"local_address":  "10.1.81.16",
			"local_port":     33061,
			"mysql_port":     3306,
			"server_id":      1,
			"primary_host":   "10.1.81.16",
			"primary_port":   3306,
			"replicate_user": "repl_user",
			"replicate_pass": "Repl@123456",
			"mysql_user":     "root",
			"mysql_password": "",
			"bootstrap":      true,
		},
	}

	body, _ := json.Marshal(deployReq)

	req, _ := http.NewRequest("POST", agentURL+"/agent/tasks/deploy", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer dev-agent-token-CHANGE-ME-at-least-16")
	req.Header.Set("Content-Type", "application/json")

	fmt.Println("Testing Agent MGR deploy directly...")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Response (%d):\n%s\n", resp.StatusCode, string(respBody))

	// 分析结果
	var result struct {
		Data struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"data"`
	}
	json.Unmarshal(respBody, &result)

	fmt.Printf("\n--- Analysis ---\n")
	fmt.Printf("Status: %s\n", result.Data.Status)
	fmt.Printf("Message: %s\n", result.Data.Message)

	if result.Data.Message != "" {
		// 检查是否还是mysql命令找不到错误
		if contains(result.Data.Message, "executable file not found") {
			fmt.Println("\n❌ STILL FAILING: mysql command not found")
		} else if contains(result.Data.Message, "Access denied") || contains(result.Data.Message, "Can't connect") {
			fmt.Println("\n✅ mysql command WORKS! (failure is due to credentials/connection)")
		} else {
			fmt.Println("\n⚠️ Different error - mysql command issue resolved")
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
