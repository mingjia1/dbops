package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const backendURL = "http://localhost:8080"

func getToken() string {
	body, _ := json.Marshal(map[string]string{"username": "codexadmin131813", "password": "admin123"})
	resp, _ := http.Post(backendURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
	defer resp.Body.Close()
	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Data.Token
}

func main() {
	token := getToken()
	fmt.Printf("Token: %s...\n", token[:30])

	// 获取16的host_id
	req1, _ := http.NewRequest("GET", backendURL+"/api/v1/hosts", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	resp1, _ := http.DefaultClient.Do(req1)
	defer resp1.Body.Close()

	var hosts struct {
		Data []struct {
			ID      string `json:"id"`
			Address string `json:"address"`
		} `json:"data"`
	}
	json.NewDecoder(resp1.Body).Decode(&hosts)

	var host16ID string
	for _, h := range hosts.Data {
		if h.Address == "10.1.81.16" {
			host16ID = h.ID
			break
		}
	}

	fmt.Printf("Host 16 ID: %s\n\n", host16ID)

	// 部署单节点MGR（16主机，MySQL 8.0）
	fmt.Println("=== Deploying Single-Node MGR on 16 (MySQL 8.0) ===")

	deployReq := map[string]interface{}{
		"cluster_id":       "mgr-single-16",
		"name":             "Single Node MGR Test",
		"primary_host_id":  host16ID,
		"primary_port":     3307,
		"replica_host_ids": []string{},
		"replica_port":     3307,
	}

	deployBody, _ := json.Marshal(deployReq)
	req2, _ := http.NewRequest("POST", backendURL+"/api/v1/deployments/mgr", bytes.NewBuffer(deployBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")

	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()

	respBody, _ := ioutil.ReadAll(resp2.Body)
	fmt.Printf("Response (%d):\n%s\n\n", resp2.StatusCode, string(respBody))

	// 监控部署
	fmt.Println("=== Monitoring deployment (max 5 minutes) ===")

	for i := 0; i < 30; i++ {
		time.Sleep(10 * time.Second)

		req3, _ := http.NewRequest("GET", backendURL+"/api/v1/deployments/mgr-single-16", nil)
		req3.Header.Set("Authorization", "Bearer "+token)
		resp3, _ := http.DefaultClient.Do(req3)

		var status struct {
			Data struct {
				Status   string `json:"status"`
				Progress int    `json:"progress"`
				Stage    string `json:"stage"`
				Message  string `json:"message"`
				Steps    []struct {
					Name    string `json:"name"`
					Status  string `json:"status"`
					Message string `json:"message"`
				} `json:"steps"`
			} `json:"data"`
		}
		json.NewDecoder(resp3.Body).Decode(&status)
		resp3.Body.Close()

		fmt.Printf("[%ds] %s (%d%%) - %s\n",
			(i+1)*10, status.Data.Status, status.Data.Progress, status.Data.Stage)

		// 显示最新步骤
		if len(status.Data.Steps) > 0 {
			lastStep := status.Data.Steps[len(status.Data.Steps)-1]
			fmt.Printf("      Last step: %s (%s)\n", lastStep.Name, lastStep.Status)
			if lastStep.Status == "failed" && lastStep.Message != "" {
				fmt.Printf("      Error: %s\n", lastStep.Message)
			}
		}

		if status.Data.Status == "completed" {
			fmt.Println("\n✅ Deployment SUCCESS!")
			break
		}
		if status.Data.Status == "failed" {
			fmt.Printf("\n❌ Deployment FAILED\n")
			fmt.Printf("Message: %s\n", status.Data.Message)

			// 显示所有失败步骤
			fmt.Println("\nFailed steps:")
			for _, step := range status.Data.Steps {
				if step.Status == "failed" {
					fmt.Printf("  - %s: %s\n", step.Name, step.Message)
				}
			}
			break
		}
	}
}
