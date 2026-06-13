package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func main() {
	// 部署3节点MGR: 16主 + 17副本1 + 17副本2
	clusterID := fmt.Sprintf("mgr-test-%d", time.Now().Unix())

	token := "Bearer dev-agent-token-CHANGE-ME-at-least-16"

	// 1. 在16上部署主节点
	fmt.Println("=== 1. 部署主节点 (16:3306) ===")
	primary := map[string]interface{}{
		"config": map[string]interface{}{
			"port":          3306,
			"data_dir":      "/data/mysql/3306",
			"mysql_user":    "root",
			"mysql_pass":    "root",
			"server_id":     3306,
			"install_type":  "mgr",
			"is_primary":    true,
			"group_name":    clusterID,
			"local_address": "10.1.81.16:33061",
		},
	}

	if !deploy("http://10.1.81.16:9090/agent/tasks/deploy", primary, token) {
		fmt.Println("❌ 主节点部署失败")
		return
	}

	time.Sleep(2 * time.Second)

	// 2. 在17上部署副本1
	fmt.Println("\n=== 2. 部署副本1 (17:3307) ===")
	replica1 := map[string]interface{}{
		"config": map[string]interface{}{
			"port":          3307,
			"data_dir":      "/data/mysql/3307",
			"mysql_user":    "root",
			"mysql_pass":    "root",
			"server_id":     3307,
			"install_type":  "mgr",
			"is_primary":    false,
			"group_name":    clusterID,
			"local_address": "10.1.81.17:33072",
			"seeds":         "10.1.81.16:33061",
		},
	}

	if !deploy("http://10.1.81.17:9090/agent/tasks/deploy", replica1, token) {
		fmt.Println("❌ 副本1部署失败")
		return
	}

	time.Sleep(2 * time.Second)

	// 3. 在17上部署副本2
	fmt.Println("\n=== 3. 部署副本2 (17:3308) ===")
	replica2 := map[string]interface{}{
		"config": map[string]interface{}{
			"port":          3308,
			"data_dir":      "/data/mysql/3308",
			"mysql_user":    "root",
			"mysql_pass":    "root",
			"server_id":     3308,
			"install_type":  "mgr",
			"is_primary":    false,
			"group_name":    clusterID,
			"local_address": "10.1.81.17:33082",
			"seeds":         "10.1.81.16:33061,10.1.81.17:33072",
		},
	}

	if !deploy("http://10.1.81.17:9090/agent/tasks/deploy", replica2, token) {
		fmt.Println("❌ 副本2部署失败")
		return
	}

	fmt.Println("\n✅ 3节点MGR集群部署完成")
	fmt.Printf("集群ID: %s\n", clusterID)
}

func deploy(url string, payload map[string]interface{}, token string) bool {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("HTTP状态: %d\n", resp.StatusCode)
	fmt.Printf("响应: %s\n", string(respBody))

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	data, _ := result["data"].(map[string]interface{})
	status, _ := data["status"].(string)
	message, _ := data["message"].(string)

	fmt.Printf("状态: %s\n", status)
	fmt.Printf("消息: %s\n", message)

	return status == "completed"
}
