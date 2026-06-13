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
		Data struct{ Token string `json:"token"` } `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Data.Token
}

func main() {
	token := getToken()
	fmt.Printf("Token: %s...\n\n", token[:20])

	// 清理旧记录
	for _, id := range []string{"mgr-prod-16", "mgr-16-final"} {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/deployments/%s", backendURL, id), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil { resp.Body.Close() }
	}
	time.Sleep(2 * time.Second)

	// 获取16主机ID
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
		}
	}
	fmt.Printf("Host 16 ID: %s\n\n", host16ID)

	// 部署MGR - 使用3306端口和正确密码
	fmt.Println("=== Deploying MGR on 16 (port 3306, MySQL 8.0) ===")
	deployReq := map[string]interface{}{
		"cluster_id":       "mgr-16-final",
		"name":             "MGR Final Test",
		"primary_host_id":  host16ID,
		"primary_port":     3306,
		"replica_host_ids": []string{},
		"replica_port":     3306,
		"mysql_user":       "root",
		"mysql_password":   "root",
	}
	deployBody, _ := json.Marshal(deployReq)
	req2, _ := http.NewRequest("POST", backendURL+"/api/v1/deployments/mgr", bytes.NewBuffer(deployBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()
	respBody, _ := ioutil.ReadAll(resp2.Body)
	fmt.Printf("Response (%d):\n%s\n\n", resp2.StatusCode, string(respBody))

	// 监控
	fmt.Println("=== Monitoring ===")
	for i := 0; i < 24; i++ {
		time.Sleep(10 * time.Second)
		req3, _ := http.NewRequest("GET", backendURL+"/api/v1/deployments/mgr-16-final", nil)
		req3.Header.Set("Authorization", "Bearer "+token)
		resp3, _ := http.DefaultClient.Do(req3)
		var status struct {
			Data struct {
				Status   string `json:"status"`
				Progress int    `json:"progress"`
				Stage    string `json:"stage"`
				Steps    []struct {
					Name    string `json:"name"`
					Status  string `json:"status"`
					Message string `json:"message"`
				} `json:"steps"`
			} `json:"data"`
		}
		json.NewDecoder(resp3.Body).Decode(&status)
		resp3.Body.Close()

		fmt.Printf("[%ds] %s (%d%%) - %s\n", (i+1)*10, status.Data.Status, status.Data.Progress, status.Data.Stage)
		if len(status.Data.Steps) > 0 {
			last := status.Data.Steps[len(status.Data.Steps)-1]
			fmt.Printf("      → %s (%s) %s\n", last.Name, last.Status, last.Message)
		}

		if status.Data.Status == "completed" {
			fmt.Println("\n✅✅✅ MGR DEPLOYMENT SUCCESS! ✅✅✅")
			for _, s := range status.Data.Steps {
				fmt.Printf("  ✓ %s\n", s.Name)
			}
			return
		}
		if status.Data.Status == "failed" {
			fmt.Printf("\n❌ FAILED\n")
			for _, s := range status.Data.Steps {
				if s.Status == "failed" {
					fmt.Printf("  ✗ %s: %s\n", s.Name, s.Message)
				}
			}
			return
		}
	}
	fmt.Println("\n⏱️ Timeout")
}
