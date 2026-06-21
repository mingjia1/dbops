package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- parsePXCConfig ---

func TestParsePXCConfig_Defaults(t *testing.T) {
	got := parsePXCConfig(map[string]interface{}{})
	assert.Equal(t, "pxc-cluster", got.ClusterName)
	assert.Equal(t, 3306, got.MySQLPort)
	assert.Equal(t, 4567, got.WSREPPort)
	assert.Equal(t, "xtrabackup-v2", got.SSTMethod)
	assert.Equal(t, "sstuser", got.ReplicateUser)
	assert.Equal(t, "sstpass", got.ReplicatePass)
	assert.Equal(t, "/var/lib/mysql", got.DataDir)
	assert.False(t, got.Bootstrap)
	assert.Equal(t, 4444, got.WSREPSSTPort)
	assert.False(t, got.WSREPSSLEnabled)
	assert.Equal(t, "root", got.MySQLUser)
	assert.Empty(t, got.Nodes)
	assert.Empty(t, got.NodeHost)
	assert.Empty(t, got.MySQLPassword)
	assert.Empty(t, got.MySQLVersion)
}

func TestParsePXCConfig_CustomValues(t *testing.T) {
	got := parsePXCConfig(map[string]interface{}{
		"cluster_name":       "pxc-prod",
		"mysql_port":         3307,
		"wsrep_port":         4568,
		"sst_method":         "xtrabackup",
		"replicate_user":     "sst",
		"replicate_pass":     "sst!@#",
		"data_dir":           "/data/mysql/pxc-3307",
		"bootstrap":          true,
		"wsrep_sst_port":     4445,
		"wsrep_ssl_enabled":  true,
		"mysql_user":         "admin",
		"mysql_password":     "admin!@#",
		"node_host":          "10.0.0.1",
		"nodes":              []interface{}{"10.0.0.1", "10.0.0.2"},
	})
	assert.Equal(t, "pxc-prod", got.ClusterName)
	assert.Equal(t, 3307, got.MySQLPort)
	assert.Equal(t, 4568, got.WSREPPort)
	assert.Equal(t, "xtrabackup", got.SSTMethod)
	assert.Equal(t, "sst", got.ReplicateUser)
	assert.Equal(t, "sst!@#", got.ReplicatePass)
	assert.Equal(t, "/data/mysql/pxc-3307", got.DataDir)
	assert.True(t, got.Bootstrap)
	assert.Equal(t, 4445, got.WSREPSSTPort)
	assert.True(t, got.WSREPSSLEnabled)
	assert.Equal(t, "admin", got.MySQLUser)
	assert.Equal(t, "admin!@#", got.MySQLPassword)
	assert.Equal(t, "10.0.0.1", got.NodeHost)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, got.Nodes)
}

func TestParsePXCConfig_NodesAsStringSlice(t *testing.T) {
	got := parsePXCConfig(map[string]interface{}{
		"nodes": []string{"10.0.0.1", "10.0.0.2"},
	})
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, got.Nodes)
}

func TestParsePXCConfig_WrongTypes(t *testing.T) {
	got := parsePXCConfig(map[string]interface{}{
		"mysql_port":   "abc",
		"wsrep_port":   false,
		"bootstrap":    "yes",
		"wsrep_sst_port": "def",
	})
	assert.Equal(t, 3306, got.MySQLPort)
	assert.Equal(t, 4567, got.WSREPPort)
	assert.False(t, got.Bootstrap)
	assert.Equal(t, 4444, got.WSREPSSTPort)
}

// --- PXC MySQLVersion parsing (MySQL 5.7 specific) ---

func TestParsePXCConfig_MySQLVersion57(t *testing.T) {
	got := parsePXCConfig(map[string]interface{}{
		"mysql_version": "5.7",
	})
	assert.Equal(t, "5.7", got.MySQLVersion)
}

func TestParsePXCConfig_MySQLVersion80(t *testing.T) {
	got := parsePXCConfig(map[string]interface{}{
		"mysql_version": "8.0",
	})
	assert.Equal(t, "8.0", got.MySQLVersion)
}

func TestParsePXCConfig_MySQLVersionEmpty(t *testing.T) {
	got := parsePXCConfig(map[string]interface{}{})
	assert.Empty(t, got.MySQLVersion)
}

func TestParsePXCConfig_MySQLVersionWrongType(t *testing.T) {
	got := parsePXCConfig(map[string]interface{}{
		"mysql_version": 5.7,
	})
	assert.Empty(t, got.MySQLVersion)
}

// --- safeNodeName ---

func TestSafeNodeName_DotsReplaced(t *testing.T) {
	assert.Equal(t, "10-0-0-1", safeNodeName("10.0.0.1"))
}

func TestSafeNodeName_ColonsReplaced(t *testing.T) {
	assert.Equal(t, "host-1", safeNodeName("host:1"))
}

func TestSafeNodeName_PlainHost(t *testing.T) {
	assert.Equal(t, "db-host", safeNodeName("db-host"))
}

// --- pxcOptionSafePassword ---

func TestPxcOptionSafePassword_NoChange(t *testing.T) {
	assert.Equal(t, "normalpass", pxcOptionSafePassword("normalpass"))
}

func TestPxcOptionSafePassword_RemovesHash(t *testing.T) {
	assert.Equal(t, "pass_with_underscore", pxcOptionSafePassword("pass_with#underscore"))
}

func TestPxcOptionSafePassword_RemovesNewlines(t *testing.T) {
	assert.Equal(t, "pass_with__", pxcOptionSafePassword("pass_with\n\r"))
}

func TestPxcOptionSafePassword_Empty(t *testing.T) {
	assert.Equal(t, "", pxcOptionSafePassword(""))
}

// --- filepathJoin ---

func TestFilepathJoin_Simple(t *testing.T) {
	assert.Equal(t, "/data/mysql/mysql.sock", filepathJoin("/data/mysql", "mysql.sock"))
}

func TestFilepathJoin_TrailingSlash(t *testing.T) {
	assert.Equal(t, "/data/mysql/mysql.sock", filepathJoin("/data/mysql/", "mysql.sock"))
}

func TestFilepathJoin_EmptyBase(t *testing.T) {
	assert.Equal(t, "file.txt", filepathJoin("", "file.txt"))
}

// --- pxcConfigPath, pxcInitConfigPath, pxcErrorLogPath, pxcStartupLogPath ---

func TestPXCConfigPath(t *testing.T) {
	config := PXCConfig{MySQLPort: 3306}
	assert.Contains(t, pxcConfigPath(config), "dbops-pxc-3306.cnf")
}

func TestPXCInitConfigPath(t *testing.T) {
	config := PXCConfig{MySQLPort: 3307}
	assert.Contains(t, pxcInitConfigPath(config), "dbops-pxc-3307-init.cnf")
}

func TestPXCEerrorLogPath(t *testing.T) {
	config := PXCConfig{MySQLPort: 3306}
	assert.Contains(t, pxcErrorLogPath(config), "pxc-3306-error.log")
}

func TestPXCStartupLogPath(t *testing.T) {
	config := PXCConfig{MySQLPort: 3306}
	assert.Contains(t, pxcStartupLogPath(config), "pxc-3306-startup.log")
}

// --- PXCConfig struct ---

func TestPXCConfig_Fields(t *testing.T) {
	c := PXCConfig{
		ClusterName:     "pxc-test",
		Nodes:           []string{"node1", "node2"},
		MySQLPort:       3306,
		WSREPPort:       4567,
		SSTMethod:       "xtrabackup-v2",
		ReplicateUser:   "sstuser",
		ReplicatePass:   "sstpass",
		DataDir:         "/data/mysql/pxc-3306",
		Bootstrap:       true,
		WSREPSSTPort:    4444,
		WSREPSSLEnabled: false,
		MySQLUser:       "root",
		MySQLPassword:   "rootpass",
		NodeHost:        "10.0.0.1",
		MySQLVersion:    "5.7",
	}
	assert.Equal(t, "pxc-test", c.ClusterName)
	assert.Len(t, c.Nodes, 2)
	assert.True(t, c.Bootstrap)
	assert.Equal(t, "rootpass", c.MySQLPassword)
	assert.Equal(t, "5.7", c.MySQLVersion)
}

// --- PXCSyncStatus struct ---

func TestPXCSyncStatus_Fields(t *testing.T) {
	s := PXCSyncStatus{
		ClusterSize:       3,
		ClusterStatus:     "Primary",
		LocalIndex:        0,
		LocalState:        "Synced",
		Ready:             true,
		FlowControlPaused: false,
		InnoDBBufferPool:  "1G",
	}
	assert.Equal(t, 3, s.ClusterSize)
	assert.Equal(t, "Primary", s.ClusterStatus)
	assert.True(t, s.Ready)
}

// --- DeployPXC (struct-level, no mysql) ---

func TestDeployPXC_MissingNodes(t *testing.T) {
	e := &TaskExecutor{}
	// No nodes config should fail with "requires at least one node"
	result, err := e.DeployPXC(nil, DeployTaskRequest{
		TaskID: "test-pxc",
		Config: map[string]interface{}{},
	})
	// Note: nil context will panic if code reaches sql execution, but missing nodes check should happen first
	if err != nil {
		assert.Contains(t, err.Error(), "requires")
	} else if result != nil {
		assert.Equal(t, "failed", result.Status)
	}
}

func TestDeployPXC_MissingPassword(t *testing.T) {
	e := &TaskExecutor{}
	result, err := e.DeployPXC(nil, DeployTaskRequest{
		TaskID: "test-pxc",
		Config: map[string]interface{}{
			"nodes": []interface{}{"10.0.0.1"},
		},
	})
	if err != nil {
		assert.Contains(t, err.Error(), "password")
	} else if result != nil {
		assert.Equal(t, "failed", result.Status)
	}
}

// --- buildPXCConfigContent (does not require mysql) ---

func TestBuildPXCConfigContent_Basic(t *testing.T) {
	config := PXCConfig{
		ClusterName: "pxc-test",
		Nodes:       []string{"10.0.0.1", "10.0.0.2"},
		MySQLPort:   3306,
		WSREPPort:   4567,
		SSTMethod:   "xtrabackup-v2",
		DataDir:     "/data/mysql/pxc-3306",
		NodeHost:    "10.0.0.1",
		MySQLConfig: map[string]string{},
	}
	content := buildPXCConfigContent(config)
	assert.Contains(t, content, "pxc-test")
	assert.Contains(t, content, "gcomm://")
	assert.Contains(t, content, "10.0.0.1:4567")
	assert.Contains(t, content, "10.0.0.2:4567")
	assert.Contains(t, content, "xtrabackup-v2")
}
