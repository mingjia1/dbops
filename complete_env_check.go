package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	backendURL = "http://localhost:8080"
	token      = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiZTRmM2E1YzMtNThkNy00MzA1LWFmMGItMzVmMWU0OGEzMjcxIiwidXNlcm5hbWUiOiJjb2RleGFkbWluMTMxODEzIiwicm9sZSI6ImFkbWluIiwiZXhwIjoxNzgxMzQ1NDEwLCJuYmYiOjE3ODEyNTkwMTAsImlhdCI6MTc4MTI1OTAxMH0.Ul6GeZNMkaZCaeB_15gA4XJD4CunCq3Q2ijm4pWLQwQ"
	agentToken = "dev-agent-token-CHANGE-ME-at-least-16"
)

type Host struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Status  string `json:"status"`
}

type AgentEnvCheckResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Status  string                 `json:"status"`
		Message string                 `json:"message"`
		OS      map[string]interface{} `json:"os"`
		Tools   map[string]interface{} `json:"tools"`
	} `json:"data"`
}

func main() {
	fmt.Println("=== Complete Environment Check Test ===\n")

	// 1. 获取主机列表
	hosts, err := getHosts()
	if err != nil {
		fmt.Printf("ERROR: Failed to get hosts: %v\n", err)
		return
	}

	if len(hosts) == 0 {
		fmt.Println("No hosts found")
		return
	}

	fmt.Printf("Found %d hosts\n\n", len(hosts))

	// 2. 选择一台成功状态的主机进行测试
	var testHost *Host
	for i := range hosts {
		if hosts[i].Status == "success" {
			testHost = &hosts[i]
			break
		}
	}

	if testHost == nil {
		fmt.Println("No healthy host found for testing")
		return
	}

	fmt.Printf("Selected host for testing:\n")
	fmt.Printf("  ID: %s\n", testHost.ID)
	fmt.Printf("  Name: %s\n", testHost.Name)
	fmt.Printf("  Address: %s\n", testHost.Address)
	fmt.Printf("  Status: %s\n\n", testHost.Status)

	// 3. 测试Agent当前版本
	fmt.Println("Step 1: Testing current Agent version...")
	hasEnvCheck := testAgentEnvCheck(testHost.Address)

	if hasEnvCheck {
		fmt.Println("✓ Agent already has environment check API!\n")
		fmt.Println("=== Test Complete - Feature Already Available ===")
		return
	}

	fmt.Println("✗ Agent does not have environment check API")
	fmt.Println("  Updating Agent to new version...\n")

	// 4. 更新Agent
	fmt.Println("Step 2: Updating Agent...")
	if err := updateAgent(testHost.ID); err != nil {
		fmt.Printf("ERROR: Failed to update agent: %v\n", err)
		return
	}
	fmt.Println("✓ Agent update submitted\n")

	// 5. 等待Agent重启
	fmt.Println("Step 3: Waiting for Agent to restart (60 seconds)...")
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)
		if i%10 == 0 && i > 0 {
			fmt.Printf("  %d seconds elapsed...\n", i)
		}
	}
	fmt.Println("✓ Wait complete\n")

	// 6. 测试新API
	fmt.Println("Step 4: Testing environment check API...")
	if testAgentEnvCheck(testHost.Address) {
		fmt.Println("✓ Environment check API works!\n")

		// 7. 显示完整结果
		fmt.Println("Step 5: Fetching full environment report...")
		showEnvCheckResult(testHost.Address)

		fmt.Println("\n=== Test Complete - Success! ===")
	} else {
		fmt.Println("✗ Environment check API still not available")
		fmt.Println("  Agent may need more time to restart, or update failed")
		fmt.Println("\n=== Test Complete - Failed ===")
	}
}

func getHosts() ([]Host, error) {
	req, _ := http.NewRequest("GET", backendURL+"/api/v1/hosts", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Data []Host `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func updateAgent(hostID string) error {
	payload := map[string]interface{}{
		"action":     "update",
		"agent_port": 9090,
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/hosts/%s/agent", backendURL, hostID), bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update failed: %s", string(body))
	}

	return nil
}

func testAgentEnvCheck(address string) bool {
	payload := map[string]interface{}{
		"check_tools":     true,
		"check_resources": true,
		"check_network":   true,
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s:9090/agent/tasks/check-environment", address), bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+agentToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func showEnvCheckResult(address string) {
	payload := map[string]interface{}{
		"check_tools":     true,
		"check_resources": true,
		"check_network":   true,
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s:9090/agent/tasks/check-environment", address), bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+agentToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result AgentEnvCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}

	fmt.Printf("  Status: %s\n", result.Data.Status)
	fmt.Printf("  Message: %s\n", result.Data.Message)

	if result.Data.OS != nil {
		fmt.Printf("\n  OS Information:\n")
		for k, v := range result.Data.OS {
			fmt.Printf("    %s: %v\n", k, v)
		}
	}

	if result.Data.Tools != nil {
		fmt.Printf("\n  Tools Information:\n")
		for k, v := range result.Data.Tools {
			fmt.Printf("    %s: %v\n", k, v)
		}
	}
}
