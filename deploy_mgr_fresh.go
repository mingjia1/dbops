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
	fmt.Printf("Token: %s...\n\n", token[:30])

	// 删除旧的失败部署
	fmt.Println("=== Cleaning old deployments ===")
	oldIDs := []string{"test-mgr-16", "mgr-single-16"}
	for _, id := range oldIDs {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/deployments/%s", backendURL, id), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			body, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("Delete %s: %s\n", id, string(body))
		}
	}

	time.Sleep(2 * time.Second)

	// 获取主机ID
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

	var host16ID, host17ID string
	for _, h := range hosts.Data {
		if h.Address == "10.1.81.16" {
			host16ID = h.ID
		}
		if h.Address == "10.1.81.17" {
			host17ID = h.ID
		}
	}

	fmt.Printf("\nHost 16 ID: %s\n", host16ID)
	fmt.Printf("Host 17 ID: %s\n\n", host17ID)

	// 部署新的MGR集群（只用16，因为17是MySQL 5.7，版本不兼容）
	fmt.Println("=== Deploying MGR Cluster on 16 (single node) ===")

	deployReq := map[string]interface{}{
		"cluster_id":       "mgr-prod-16",
		"name":             "Production MGR on 16",
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
	fmt.Println("=== Monitoring deployment ===")

	for i := 0; i < 30; i++ {
		time.Sleep(10 * time.Second)

		req3, _ := http.NewRequest("GET", backendURL+"/api/v1/deployments/mgr-prod-16", nil)
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

		if status.Data.Status == "completed" {
			fmt.Println("\n✅ ✅ ✅ DEPLOYMENT SUCCESS! ✅ ✅ ✅")
			fmt.Println("\nCompleted steps:")
			for _, step := range status.Data.Steps {
				if step.Status == "completed" {
					fmt.Printf("  ✓ %s\n", step.Name)
				}
			}
			return
		}

		if status.Data.Status == "failed" {
			fmt.Printf("\n❌ Deployment FAILED\n")
			fmt.Printf("Message: %s\n\n", status.Data.Message)

			fmt.Println("Failed steps:")
			for _, step := range status.Data.Steps {
				if step.Status == "failed" {
					fmt.Printf("  ✗ %s: %s\n", step.Name, step.Message)
				}
			}
			return
		}
	}

	fmt.Println("\n⏱️ Monitoring timeout after 5 minutes")
}
