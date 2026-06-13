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
	loginReq := map[string]string{"username": "codexadmin131813", "password": "admin123"}
	body, _ := json.Marshal(loginReq)
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

	// 1. 查看所有部署
	fmt.Println("=== Checking existing deployments ===")
	req1, _ := http.NewRequest("GET", backendURL+"/api/v1/deployments", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	resp1, _ := http.DefaultClient.Do(req1)
	defer resp1.Body.Close()

	var deployments struct {
		Data []struct {
			ID          string `json:"id"`
			ClusterType string `json:"cluster_type"`
			Name        string `json:"name"`
			Status      string `json:"status"`
			Progress    int    `json:"progress"`
			Message     string `json:"message"`
		} `json:"data"`
	}
	json.NewDecoder(resp1.Body).Decode(&deployments)

	if len(deployments.Data) == 0 {
		fmt.Println("No deployments found.")
	} else {
		for _, d := range deployments.Data {
			fmt.Printf("- ID: %s\n  Type: %s\n  Name: %s\n  Status: %s (%d%%)\n  Message: %s\n\n",
				d.ID, d.ClusterType, d.Name, d.Status, d.Progress, d.Message)
		}
	}

	// 2. 检查16/17主机状态
	fmt.Println("=== Checking hosts 16/17 ===")
	req2, _ := http.NewRequest("GET", backendURL+"/api/v1/hosts", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()

	var hosts struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Address string `json:"address"`
			Status  string `json:"status"`
		} `json:"data"`
	}
	json.NewDecoder(resp2.Body).Decode(&hosts)

	for _, h := range hosts.Data {
		if h.Address == "10.1.81.16" || h.Address == "10.1.81.17" {
			fmt.Printf("Host: %s (%s)\n  ID: %s\n  Status: %s\n\n", h.Address, h.Name, h.ID, h.Status)
		}
	}

	// 3. 检查16/17的Agent和MySQL状态
	fmt.Println("=== Checking Agent health ===")
	for _, addr := range []string{"10.1.81.16", "10.1.81.17"} {
		resp, err := http.Get(fmt.Sprintf("http://%s:9090/health", addr))
		if err != nil {
			fmt.Printf("%s: ❌ Agent not reachable\n", addr)
			continue
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("%s: ✅ Agent healthy - %s\n", addr, string(body))
	}
}
