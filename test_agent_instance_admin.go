package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	// 直接测试16的Agent instance-admin API
	agentURL := "http://10.1.81.16:9090"

	testReq := map[string]interface{}{
		"task_id": "test-mysql-cmd",
		"config": map[string]interface{}{
			"action":   "list_users",
			"host":     "127.0.0.1",
			"port":     3306,
			"user":     "root",
			"password": "",
		},
	}

	body, _ := json.Marshal(testReq)

	req, _ := http.NewRequest("POST", agentURL+"/agent/tasks/instance-admin", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer dev-agent-token-CHANGE-ME-at-least-16")
	req.Header.Set("Content-Type", "application/json")

	fmt.Println("Testing Agent instance-admin API (list_users)...")
	fmt.Printf("Request: %s\n\n", string(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Response (%d):\n%s\n", resp.StatusCode, string(respBody))

	// 解析响应
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"data"`
	}
	json.Unmarshal(respBody, &result)

	if result.Data.Status == "failed" {
		fmt.Printf("\n❌ FAILED: %s\n", result.Data.Message)
	} else {
		fmt.Printf("\n✅ SUCCESS\n")
	}
}
