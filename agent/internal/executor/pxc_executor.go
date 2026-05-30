package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type PXCConfig struct {
	ClusterName     string   `json:"cluster_name"`
	Nodes           []string `json:"nodes"`
	WSREPPort       int      `json:"wsrep_port"`
	SSTMethod       string   `json:"sst_method"`
	ReplicateUser   string   `json:"replicate_user"`
	ReplicatePass   string   `json:"replicate_pass"`
	DataDir         string   `json:"data_dir"`
	Bootstrap       bool     `json:"bootstrap"`
	WSREPSSTPort    int      `json:"wsrep_sst_port"`
	WSREPSSLEnabled bool     `json:"wsrep_ssl_enabled"`
}

type GaleraConfig struct {
	WSREPClusterAddress string `json:"wsrep_cluster_address"`
	WSREPNodeAddress    string `json:"wsrep_node_address"`
	WSREPNodeName       string `json:"wsrep_node_name"`
	WSREPSSTMethod      string `json:"wsrep_sst_method"`
	WSREPProvider        string `json:"wsrep_provider"`
	GCacheSize          string `json:"gcache_size"`
	GCachePageSize      string `json:"gcache_page_size"`
	GCacheKeepPagesSize string `json:"gcache_keep_pages_size"`
}

type PXCSyncStatus struct {
	ClusterSize      int    `json:"cluster_size"`
	ClusterStatus    string `json:"cluster_status"`
	LocalIndex       int    `json:"local_index"`
	LocalState       string `json:"local_state"`
	Ready            bool   `json:"ready"`
	FlowControlPaused bool  `json:"flow_control_paused"`
	InnoDBBufferPool string `json:"innodb_buffer_pool"`
}

type SplitBrainInfo struct {
	Detected       bool     `json:"detected"`
	IsolatedNodes  []string `json:"isolated_nodes"`
	ActiveNodes    []string `json:"active_nodes"`
	QuorumStatus   string   `json:"quorum_status"`
	PrimaryComponent bool   `json:"primary_component"`
}

func parsePXCConfig(config map[string]interface{}) PXCConfig {
	pc := PXCConfig{
		ClusterName:     "pxc-cluster",
		WSREPPort:       4567,
		SSTMethod:       "xtrabackup-v2",
		ReplicateUser:   "sstuser",
		ReplicatePass:   "sstpass",
		DataDir:         "/var/lib/mysql",
		Bootstrap:       false,
		WSREPSSTPort:    4444,
		WSREPSSLEnabled: false,
	}

	if v, ok := config["cluster_name"].(string); ok {
		pc.ClusterName = v
	}
	if v, ok := config["nodes"].([]string); ok {
		pc.Nodes = v
	}
	if v, ok := config["wsrep_port"].(int); ok {
		pc.WSREPPort = v
	}
	if v, ok := config["sst_method"].(string); ok {
		pc.SSTMethod = v
	}
	if v, ok := config["replicate_user"].(string); ok {
		pc.ReplicateUser = v
	}
	if v, ok := config["replicate_pass"].(string); ok {
		pc.ReplicatePass = v
	}
	if v, ok := config["data_dir"].(string); ok {
		pc.DataDir = v
	}
	if v, ok := config["bootstrap"].(bool); ok {
		pc.Bootstrap = v
	}
	if v, ok := config["wsrep_sst_port"].(int); ok {
		pc.WSREPSSTPort = v
	}
	if v, ok := config["wsrep_ssl_enabled"].(bool); ok {
		pc.WSREPSSLEnabled = v
	}

	return pc
}

func parseGaleraConfig(config map[string]interface{}) GaleraConfig {
	gc := GaleraConfig{
		WSREPSSTMethod:      "xtrabackup-v2",
		WSREPProvider:       "/usr/lib/galera4/libgalera_smm.so",
		GCacheSize:          "1G",
		GCachePageSize:      "1G",
		GCacheKeepPagesSize: "0",
	}

	if v, ok := config["wsrep_cluster_address"].(string); ok {
		gc.WSREPClusterAddress = v
	}
	if v, ok := config["wsrep_node_address"].(string); ok {
		gc.WSREPNodeAddress = v
	}
	if v, ok := config["wsrep_node_name"].(string); ok {
		gc.WSREPNodeName = v
	}
	if v, ok := config["wsrep_sst_method"].(string); ok {
		gc.WSREPSSTMethod = v
	}
	if v, ok := config["wsrep_provider"].(string); ok {
		gc.WSREPProvider = v
	}
	if v, ok := config["gcache_size"].(string); ok {
		gc.GCacheSize = v
	}
	if v, ok := config["gcache_page_size"].(string); ok {
		gc.GCachePageSize = v
	}
	if v, ok := config["gcache_keep_pages_size"].(string); ok {
		gc.GCacheKeepPagesSize = v
	}

	return gc
}

func (e *TaskExecutor) DeployPXC(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parsePXCConfig(req.Config)

	if len(config.Nodes) == 0 {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   "PXC cluster requires at least one node",
			Timestamp: time.Now(),
		}, nil
	}

	sstUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; "+
			"GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT ON *.* TO '%s'@'localhost';",
		config.ReplicateUser, config.ReplicatePass, config.ReplicateUser,
	)

	for i, node := range config.Nodes {
		nodeCmd := exec.CommandContext(ctx, "mysql", "-h", node,
			"-P", fmt.Sprintf("%d", config.WSREPPort),
			"-u", "root", "-e", sstUserSQL)

		if err := nodeCmd.Run(); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  i * 30,
				Message:   fmt.Sprintf("Failed to create SST user on node %s: %v", node, err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	bootstrapNode := config.Nodes[0]
	if config.Bootstrap {
		bootstrapCmd := exec.CommandContext(ctx, "systemctl", "start", "mysql@bootstrap")
		if err := bootstrapCmd.Run(); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  50,
				Message:   fmt.Sprintf("Failed to bootstrap PXC node %s: %v", bootstrapNode, err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	for i := 1; i < len(config.Nodes); i++ {
		node := config.Nodes[i]
		startCmd := exec.CommandContext(ctx, "systemctl", "start", "mysql")
		if err := startCmd.Run(); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  50 + i*10,
				Message:   fmt.Sprintf("Failed to start PXC node %s: %v", node, err),
				Timestamp: time.Now(),
			}, nil
		}
		time.Sleep(3 * time.Second)
	}

	statusResult := e.MonitorPXCSync(ctx, bootstrapNode, config.WSREPPort)
	if statusResult.ClusterStatus != "Primary" || !statusResult.Ready {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("PXC cluster sync check failed: cluster_status=%s, ready=%v", statusResult.ClusterStatus, statusResult.Ready),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("PXC cluster '%s' deployed successfully with %d nodes", config.ClusterName, len(config.Nodes)),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) ConfigureGalera(ctx context.Context, nodeHost string, nodePort int, config map[string]interface{}) (*TaskResult, error) {
	gc := parseGaleraConfig(config)

	if gc.WSREPClusterAddress == "" {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "failed",
			Progress:  0,
			Message:   "wsrep_cluster_address is required for Galera configuration",
			Timestamp: time.Now(),
		}, nil
	}

	configSQL := fmt.Sprintf(
		"SET GLOBAL wsrep_cluster_address='%s'; "+
			"SET GLOBAL wsrep_node_address='%s:%d'; "+
			"SET GLOBAL wsrep_node_name='%s'; "+
			"SET GLOBAL wsrep_sst_method='%s'; "+
			"SET GLOBAL wsrep_provider='%s';",
		gc.WSREPClusterAddress, gc.WSREPNodeAddress, nodePort, gc.WSREPNodeName,
		gc.WSREPSSTMethod, gc.WSREPProvider,
	)

	cmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", configSQL)

	if err := cmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "failed",
			Progress:  30,
			Message:   fmt.Sprintf("Failed to set Galera variables: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	gcacheSQL := fmt.Sprintf(
		"SET GLOBAL wsrep_provider_options='gcache.size=%s; gcache.page_size=%s; gcache.keep_pages_size=%s';",
		gc.GCacheSize, gc.GCachePageSize, gc.GCacheKeepPagesSize,
	)

	gcacheCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", gcacheSQL)

	if err := gcacheCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Failed to set Galera gcache options: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	verifyCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW VARIABLES LIKE 'wsrep_%';")

	output, err := verifyCmd.Output()
	if err != nil {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("Failed to verify Galera configuration: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	if !strings.Contains(string(output), gc.WSREPClusterAddress) {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "warning",
			Progress:  100,
			Message:   "Galera configuration applied but verification incomplete",
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    "configure-galera",
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Galera configuration applied successfully to node %s:%d", nodeHost, nodePort),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) MonitorPXCSync(ctx context.Context, nodeHost string, nodePort int) *PXCSyncStatus {
	statusCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW STATUS LIKE 'wsrep_%';")

	output, err := statusCmd.Output()
	if err != nil {
		return &PXCSyncStatus{
			ClusterSize:   0,
			ClusterStatus: "disconnected",
			Ready:         false,
		}
	}

	outputStr := string(output)
	status := &PXCSyncStatus{}

	if strings.Contains(outputStr, "wsrep_cluster_size") {
		sizeStart := strings.Index(outputStr, "wsrep_cluster_size")
		if sizeStart != -1 {
			sizeLine := outputStr[sizeStart:]
			sizeEnd := strings.Index(sizeLine, "\n")
			if sizeEnd != -1 {
				sizeLine = sizeLine[:sizeEnd]
				var size int
				fmt.Sscanf(sizeLine, "wsrep_cluster_size\t%d", &size)
				status.ClusterSize = size
			}
		}
	}

	if strings.Contains(outputStr, "wsrep_cluster_status") {
		statusStart := strings.Index(outputStr, "wsrep_cluster_status")
		if statusStart != -1 {
			statusLine := outputStr[statusStart:]
			statusEnd := strings.Index(statusLine, "\n")
			if statusEnd != -1 {
				statusLine = statusLine[:statusEnd]
				var clusterStatus string
				fmt.Sscanf(statusLine, "wsrep_cluster_status\t%s", &clusterStatus)
				status.ClusterStatus = clusterStatus
			}
		}
	}

	if strings.Contains(outputStr, "wsrep_local_index") {
		indexStart := strings.Index(outputStr, "wsrep_local_index")
		if indexStart != -1 {
			indexLine := outputStr[indexStart:]
			indexEnd := strings.Index(indexLine, "\n")
			if indexEnd != -1 {
				indexLine = indexLine[:indexEnd]
				var localIndex int
				fmt.Sscanf(indexLine, "wsrep_local_index\t%d", &localIndex)
				status.LocalIndex = localIndex
			}
		}
	}

	if strings.Contains(outputStr, "wsrep_local_state") {
		stateStart := strings.Index(outputStr, "wsrep_local_state")
		if stateStart != -1 {
			stateLine := outputStr[stateStart:]
			stateEnd := strings.Index(stateLine, "\n")
			if stateEnd != -1 {
				stateLine = stateLine[:stateEnd]
				var localState string
				fmt.Sscanf(stateLine, "wsrep_local_state\t%s", &localState)
				status.LocalState = localState
			}
		}
	}

	if strings.Contains(outputStr, "wsrep_ready") {
		if strings.Contains(outputStr, "wsrep_ready\tON") {
			status.Ready = true
		}
	}

	if strings.Contains(outputStr, "wsrep_flow_control_paused") {
		fcStart := strings.Index(outputStr, "wsrep_flow_control_paused")
		if fcStart != -1 {
			fcLine := outputStr[fcStart:]
			fcEnd := strings.Index(fcLine, "\n")
			if fcEnd != -1 {
				fcLine = fcLine[:fcEnd]
				var fcPaused string
				fmt.Sscanf(fcLine, "wsrep_flow_control_paused\t%s", &fcPaused)
				status.FlowControlPaused = (fcPaused != "0.000000")
			}
		}
	}

	return status
}

func (e *TaskExecutor) DetectSplitBrain(ctx context.Context, nodeHost string, nodePort int) (*SplitBrainInfo, error) {
	splitInfo := &SplitBrainInfo{
		Detected:        false,
		IsolatedNodes:   []string{},
		ActiveNodes:     []string{},
		QuorumStatus:    "unknown",
		PrimaryComponent: false,
	}

	statusCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW STATUS LIKE 'wsrep_cluster_status';")

	statusOutput, err := statusCmd.Output()
	if err != nil {
		splitInfo.QuorumStatus = "disconnected"
		return splitInfo, nil
	}

	statusStr := string(statusOutput)
	if strings.Contains(statusStr, "Non-Primary") {
		splitInfo.Detected = true
		splitInfo.QuorumStatus = "non-primary"
		splitInfo.PrimaryComponent = false
	} else if strings.Contains(statusStr, "Primary") {
		splitInfo.QuorumStatus = "primary"
		splitInfo.PrimaryComponent = true
	}

	sizeCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW STATUS LIKE 'wsrep_cluster_size';")

	sizeOutput, err := sizeCmd.Output()
	if err != nil {
		return splitInfo, nil
	}

	var actualClusterSize int
	sizeStr := string(sizeOutput)
	fmt.Sscanf(sizeStr, "wsrep_cluster_size\t%d", &actualClusterSize)

	addressCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW VARIABLES LIKE 'wsrep_cluster_address';")

	addressOutput, err := addressCmd.Output()
	if err != nil {
		return splitInfo, nil
	}

	addressStr := string(addressOutput)
	configuredNodes := parseWSREPAddress(addressStr)

	if len(configuredNodes) > 0 && actualClusterSize < len(configuredNodes) {
		splitInfo.Detected = true
		splitInfo.IsolatedNodes = identifyIsolatedNodes(configuredNodes, actualClusterSize)
	}

	stateCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW STATUS LIKE 'wsrep_local_state_comment';")

	stateOutput, err := stateCmd.Output()
	if err != nil {
		return splitInfo, nil
	}

	stateStr := string(stateOutput)
	if strings.Contains(stateStr, "Synced") {
		splitInfo.ActiveNodes = append(splitInfo.ActiveNodes, nodeHost)
	}

	if splitInfo.Detected {
		splitInfo.ActiveNodes = identifyActiveNodes(configuredNodes, splitInfo.IsolatedNodes)
	}

	return splitInfo, nil
}

func parseWSREPAddress(addressStr string) []string {
	var nodes []string

	startIdx := strings.Index(addressStr, "gcomm://")
	if startIdx == -1 {
		return nodes
	}

	addressPart := addressStr[startIdx+8:]
	endIdx := strings.Index(addressPart, "\n")
	if endIdx != -1 {
		addressPart = addressPart[:endIdx]
	}

	if addressPart == "" {
		return nodes
	}

	nodeList := strings.Split(addressPart, ",")
	for _, node := range nodeList {
		node = strings.TrimSpace(node)
		if node != "" {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func identifyIsolatedNodes(configuredNodes []string, activeSize int) []string {
	if activeSize >= len(configuredNodes) {
		return []string{}
	}

	isolatedCount := len(configuredNodes) - activeSize
	if isolatedCount > 0 && isolatedCount < len(configuredNodes) {
		return configuredNodes[activeSize:]
	}

	return []string{}
}

func identifyActiveNodes(configuredNodes []string, isolatedNodes []string) []string {
	isolatedMap := make(map[string]bool)
	for _, node := range isolatedNodes {
		isolatedMap[node] = true
	}

	var activeNodes []string
	for _, node := range configuredNodes {
		if !isolatedMap[node] {
			activeNodes = append(activeNodes, node)
		}
	}

	return activeNodes
}