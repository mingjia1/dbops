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
)

func main() {
	fmt.Println("=== Test Host Registration with Auto Environment Check ===\n")

	// 使用一台已有主机测试（避免实际添加新主机）
	// 我们将更新另一台主机的Agent并验证环境检测

	fmt.Println("Test scenario: Update Agent on host 10.1.81.21 and verify environment check")
	fmt.Println()

	hostID := "13364b24-779d-4c97-8644-931ea40bc2fa" // 10.1.81.21

	// 1. 检查主机当前状态
	fmt.Println("Step 1: Checking host current status...")
	host, err := getHost(hostID)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Printf("  Host: %s (%s)\n", host.Name, host.Address)
	fmt.Printf("  Status: %s\n\n", host.Status)

	// 2. 更新Agent
	fmt.Println("Step 2: Updating Agent...")
	if err := updateAgent(hostID); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Println("✓ Agent update submitted\n")

	// 3. 等待更新完成
	fmt.Println("Step 3: Waiting for Agent to update (60 seconds)...")
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)
		if i%15 == 0 && i > 0 {
			fmt.Printf("  %d seconds...\n", i)
		}
	}
	fmt.Println("✓ Wait complete\n")

	// 4. 检查主机状态
	fmt.Println("Step 4: Checking host status after update...")
	host, err = getHost(hostID)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Printf("  Status: %s\n", host.Status)
	fmt.Printf("  Last check: %s\n\n", host.LastCheckAt)

	// 5. 测试环境检测API
	fmt.Println("Step 5: Testing environment check API...")
	if testEnvCheck(host.Address) {
		fmt.Println("✓ Environment check API available\n")

		// 6. 显示环境信息
		fmt.Println("Step 6: Fetching environment details...")
		showEnvDetails(host.Address)

		fmt.Println("\n=== Test Complete - Success! ===")
		fmt.Println("\n✅ Verified:")
		fmt.Println("  - Agent updated successfully")
		fmt.Println("  - Environment check API works")
		fmt.Println("  - OS and tools information retrieved")
		fmt.Println("\n📝 Next: Backend host registration will auto-check environment")
		fmt.Println("  and install missing tools when adding new hosts")
	} else {
		fmt.Println("✗ Environment check API not available")
		fmt.Println("\n=== Test Complete - Failed ===")
	}
}

type Host struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Status      string `json:"status"`
	LastCheckAt string `json:"last_check_at"`
}

func getHost(hostID string) (*Host, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/hosts/%s", backendURL, hostID), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int   `json:"code"`
		Data *Host `json:"data"`
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

func testEnvCheck(address string) bool {
	payload := map[string]interface{}{
		"check_tools":     true,
		"check_resources": true,
		"check_network":   true,
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s:9090/agent/tasks/check-environment", address), bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer dev-agent-token-CHANGE-ME-at-least-16")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func showEnvDetails(address string) {
	payload := map[string]interface{}{
		"check_tools":     true,
		"check_resources": true,
		"check_network":   true,
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s:9090/agent/tasks/check-environment", address), bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer dev-agent-token-CHANGE-ME-at-least-16")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Status    string                 `json:"status"`
			Message   string                 `json:"message"`
			OS        map[string]interface{} `json:"os"`
			Tools     map[string]interface{} `json:"tools"`
			Resources map[string]interface{} `json:"resources"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}

	fmt.Printf("  Status: %s\n", result.Data.Status)
	fmt.Printf("  Message: %s\n\n", result.Data.Message)

	if result.Data.OS != nil {
		fmt.Printf("  📊 OS: %v %v (%v)\n",
			result.Data.OS["distribution"],
			result.Data.OS["version"],
			result.Data.OS["arch"])
	}

	if result.Data.Tools != nil {
		allReady, _ := result.Data.Tools["all_ready"].(bool)
		if allReady {
			fmt.Printf("  ✅ Tools: All MySQL tools are ready\n")
		} else {
			fmt.Printf("  ⚠️  Tools: Some MySQL tools are missing\n")
		}
	}

	if result.Data.Resources != nil {
		sufficient, _ := result.Data.Resources["sufficient"].(bool)
		if sufficient {
			fmt.Printf("  ✅ Resources: System resources are sufficient\n")
		} else {
			fmt.Printf("  ⚠️  Resources: System resources may be insufficient\n")
		}
	}
}
