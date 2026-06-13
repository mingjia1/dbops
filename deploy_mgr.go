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

func main() {
	// 1. Login
	loginReq := map[string]string{"username": "codexadmin131813", "password": "admin123"}
	loginBody, _ := json.Marshal(loginReq)
	loginResp, err := http.Post(backendURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(loginBody))
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		return
	}
	defer loginResp.Body.Close()

	var loginData struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.NewDecoder(loginResp.Body).Decode(&loginData)
	token := loginData.Data.Token
	fmt.Printf("Logged in, token: %s...\n", token[:30])

	// 2. Get hosts
	req, _ := http.NewRequest("GET", backendURL+"/api/v1/hosts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	hostsResp, _ := http.DefaultClient.Do(req)
	defer hostsResp.Body.Close()

	var hostsData struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"data"`
	}
	json.NewDecoder(hostsResp.Body).Decode(&hostsData)

	var host16ID string
	_ = host16ID // will be used later
	for _, h := range hostsData.Data {
		if h.Address == "10.1.81.16" {
			host16ID = h.ID
			fmt.Printf("Host 16: %s (%s)\n", h.Name, h.ID)
		}
		if h.Address == "10.1.81.17" {
			fmt.Printf("Host 17: %s (%s)\n", h.Name, h.ID)
		}
	}

	if host16ID == "" {
		fmt.Println("Host 16 not found")
		return
	}

	// 3. Deploy single-node MGR on host 16 (MySQL 8.0) in pseudo mode
	fmt.Println("\n=== Deploying single-node MGR cluster on 16 (pseudo mode) ===")
	deployReq := map[string]interface{}{
		"cluster_id":       "test-mgr-16-pseudo",
		"name":             "Test MGR on 16 (pseudo)",
		"primary_host_id":  host16ID,
		"primary_port":     3307,
		"replica_host_ids": []string{},
		"replica_port":     3307,
		"pseudo_mode":      true,
	}
	deployBody, _ := json.Marshal(deployReq)

	req2, _ := http.NewRequest("POST", backendURL+"/api/v1/deployments/mgr", bytes.NewBuffer(deployBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")

	deployResp, err := http.DefaultClient.Do(req2)
	if err != nil {
		fmt.Printf("Deploy request failed: %v\n", err)
		return
	}
	defer deployResp.Body.Close()

	deployRespBody, _ := ioutil.ReadAll(deployResp.Body)
	fmt.Printf("Deploy response (%d):\n%s\n", deployResp.StatusCode, string(deployRespBody))

	// 4. Wait and check status
	fmt.Println("\nWaiting 30 seconds for deployment...")
	time.Sleep(30 * time.Second)

	req3, _ := http.NewRequest("GET", backendURL+"/api/v1/deployments/test-mgr-16-pseudo", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	statusResp, _ := http.DefaultClient.Do(req3)
	defer statusResp.Body.Close()

	statusBody, _ := ioutil.ReadAll(statusResp.Body)
	fmt.Printf("\nDeployment status:\n%s\n", string(statusBody))
}
