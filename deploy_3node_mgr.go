package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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

	// 获取主机列表
	req, _ := http.NewRequest("GET", backendURL+"/api/v1/hosts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	var hosts struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&hosts)

	fmt.Println("=== 主机列表 ===")
	var host16, host17, host18 string
	for _, h := range hosts.Data {
		fmt.Printf("%s: %s (%s)\n", h.Address, h.ID, h.Name)
		if h.Address == "10.1.81.16" {
			host16 = h.ID
		} else if h.Address == "10.1.81.17" {
			host17 = h.ID
		} else if h.Address == "10.1.81.18" {
			host18 = h.ID
		}
	}

	fmt.Printf("\nHost 16 ID: %s\n", host16)
	fmt.Printf("Host 17 ID: %s\n", host17)
	fmt.Printf("Host 18 ID: %s\n", host18)

	// 部署3节点MGR集群
	fmt.Println("\n=== 部署3节点MGR集群 (16作为Primary, 17/18作为Secondary) ===")
	deployReq := map[string]interface{}{
		"cluster_id":        "mgr-3node-prod",
		"name":              "MGR 3-Node Cluster",
		"primary_host_id":   host16,
		"primary_port":      3306,
		"replica_host_ids":  []string{host17, host18},
		"replica_port":      3306,
		"mysql_user":        "root",
		"mysql_password":    "root",
	}

	deployBody, _ := json.Marshal(deployReq)
	req2, _ := http.NewRequest("POST", backendURL+"/api/v1/deployments/mgr", bytes.NewBuffer(deployBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()

	respBody, _ := ioutil.ReadAll(resp2.Body)
	fmt.Printf("Response (%d):\n%s\n", resp2.StatusCode, string(respBody))
}
