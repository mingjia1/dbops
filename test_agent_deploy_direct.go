package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	// 直接调用16上的Agent部署MGR
	agentURL := "http://10.1.81.16:9090"

	deployReq := map[string]interface{}{
		"type": "mgr",
		"config": map[string]interface{}{
			"cluster_name":    "test-direct",
			"local_address":   "10.1.81.16",
			"mysql_port":      3307,
			"mysql_user":      "root",
			"mysql_password":  "",
			"replicate_user":  "repl",
			"replicate_pass":  "repl123",
			"group_name":      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			"bootstrap":       true,
		},
	}

	body, _ := json.Marshal(deployReq)

	req, _ := http.NewRequest("POST", agentURL+"/agent/tasks/deploy", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer dev-agent-token-CHANGE-ME-at-least-16")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Response (%d):\n%s\n", resp.StatusCode, string(respBody))
}
