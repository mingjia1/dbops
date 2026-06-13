package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	// 直接调用16的Agent部署单实例
	deployReq := map[string]interface{}{
		"config": map[string]interface{}{
			"port":          3306,
			"data_dir":      "/data/mysql/3306",
			"mysql_user":    "root",
			"mysql_pass":    "root",
			"server_id":     3306,
			"install_type":  "mgr",
			"is_primary":    true,
			"group_name":    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"local_address": "10.1.81.16:33061",
		},
	}

	body, _ := json.Marshal(deployReq)
	fmt.Println("=== 部署请求 ===")
	fmt.Println(string(body))

	req, _ := http.NewRequest("POST", "http://10.1.81.16:9090/agent/tasks/deploy", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer dev-agent-token-CHANGE-ME-at-least-16")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("\n=== 响应 (%d) ===\n", resp.StatusCode)

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	prettyJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(prettyJSON))
}
