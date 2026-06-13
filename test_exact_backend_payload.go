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
	agentURL := "http://10.1.81.16:9090"

	// 使用与Backend完全相同的payload
	deployReq := map[string]interface{}{
		"task_id":     "backend-replica-test",
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
			"replicate_user": "repl",
			"replicate_pass": "Repl#2024!ChangeMe",
			"mysql_user":     "root",
			"mysql_password": "root", // 与Backend相同
			"bootstrap":      true,
		},
	}

	body, _ := json.Marshal(deployReq)
	req, _ := http.NewRequest("POST", agentURL+"/agent/tasks/deploy", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer dev-agent-token-CHANGE-ME-at-least-16")
	req.Header.Set("Content-Type", "application/json")

	fmt.Println("Testing Agent with Backend's exact payload...")
	fmt.Printf("Payload: %s\n\n", string(body))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Response (%d):\n%s\n", resp.StatusCode, string(respBody))

	var result struct {
		Data struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"data"`
	}
	json.Unmarshal(respBody, &result)

	if result.Data.Message != "" {
		fmt.Printf("\n---\nMessage: %s\n", result.Data.Message)
		if len(result.Data.Message) > 30 && result.Data.Message[:30] == "Failed to create replication " {
			if contains(result.Data.Message, "executable file not found") {
				fmt.Println("❌ mysql command NOT FOUND (same as Backend)")
			} else if contains(result.Data.Message, "Access denied") {
				fmt.Println("✅ mysql command works (credentials issue)")
			}
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}
