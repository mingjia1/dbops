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

	// 1. 获取16和17的host_id
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

	fmt.Printf("Host 16 ID: %s\n", host16ID)
	fmt.Printf("Host 17 ID: %s\n\n", host17ID)

	// 2. 部署MGR集群（16主 + 17从）
	fmt.Println("=== Deploying MGR Cluster (16 primary, 17 replica) ===")

	deployReq := map[string]interface{}{
		"cluster_id":       "mgr-prod-001",
		"name":             "Production MGR Cluster",
		"primary_host_id":  host16ID,
		"primary_port":     3307,
		"replica_host_ids": []string{host17ID},
		"replica_port":     3307,
	}

	deployBody, _ := json.Marshal(deployReq)
	req2, _ := http.NewRequest("POST", backendURL+"/api/v1/deployments/mgr", bytes.NewBuffer(deployBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")

	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()

	respBody, _ := ioutil.ReadAll(resp2.Body)
	fmt.Printf("Deploy Response (%d):\n%s\n\n", resp2.StatusCode, string(respBody))

	// 3. 持续监控部署状态
	fmt.Println("=== Monitoring deployment progress ===")

	for i := 0; i < 20; i++ {
		time.Sleep(10 * time.Second)

		req3, _ := http.NewRequest("GET", backendURL+"/api/v1/deployments/mgr-prod-001", nil)
		req3.Header.Set("Authorization", "Bearer "+token)
		resp3, _ := http.DefaultClient.Do(req3)

		var status struct {
			Data struct {
				Status   string `json:"status"`
				Progress int    `json:"progress"`
				Stage    string `json:"stage"`
				Message  string `json:"message"`
			} `json:"data"`
		}
		json.NewDecoder(resp3.Body).Decode(&status)
		resp3.Close()

		fmt.Printf("[%ds] Status: %s (%d%%) - %s - %s\n",
			(i+1)*10, status.Data.Status, status.Data.Progress, status.Data.Stage, status.Data.Message)

		if status.Data.Status == "completed" {
			fmt.Println("\n✅ Deployment completed successfully!")
			break
		}
		if status.Data.Status == "failed" {
			fmt.Printf("\n❌ Deployment failed: %s\n", status.Data.Message)

			// 获取详细日志
			req4, _ := http.NewRequest("GET", backendURL+"/api/v1/deployments/mgr-prod-001", nil)
			req4.Header.Set("Authorization", "Bearer "+token)
			resp4, _ := http.DefaultClient.Do(req4)
			detailBody, _ := ioutil.ReadAll(resp4.Body)
			resp4.Close()

			fmt.Printf("\nDetailed status:\n%s\n", string(detailBody))
			break
		}
	}
}
