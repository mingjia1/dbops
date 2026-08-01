package executor

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FlavorTLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CAFile   string `json:"ca_file,omitempty"`
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
}

// FlavorTaskRequest is the common Agent payload for a dedicated 信创 executor.
// PackagePath must point at a pre-staged local package bundle.
type FlavorTaskRequest struct {
	TaskID      string           `json:"task_id"`
	InstanceID  string           `json:"instance_id"`
	Flavor      string           `json:"flavor"`
	Version     string           `json:"version"`
	Operation   string           `json:"operation"`
	PackagePath string           `json:"package_path"`
	TLS         *FlavorTLSConfig `json:"tls"`
	OceanBase   *OceanBaseConfig `json:"oceanbase,omitempty"`
}

// OceanBaseConfig contains the fixed single-node inputs accepted by the
// OceanBase executor. It deliberately has no free-form command fields.
type OceanBaseConfig struct {
	ClusterName           string                  `json:"cluster_name"`
	Address               string                  `json:"address"`
	Zone                  string                  `json:"zone"`
	ClusterID             int                     `json:"cluster_id"`
	SQLPort               int                     `json:"sql_port"`
	RPCPort               int                     `json:"rpc_port"`
	DataDir               string                  `json:"data_dir"`
	SystemMemory          string                  `json:"system_memory"`
	DatafileSize          string                  `json:"datafile_size"`
	RootPassword          string                  `json:"root_password"`
	Tenant                string                  `json:"tenant"`
	TenantCPU             int                     `json:"tenant_cpu"`
	TenantMemory          string                  `json:"tenant_memory"`
	TenantLogDiskSize     string                  `json:"tenant_log_disk_size"`
	Parameters            map[string]string       `json:"parameters"`
	EnableOBProxy         bool                    `json:"enable_obproxy"`
	OBProxyPort           int                     `json:"obproxy_port"`
	OBProxySysPasswordSHA string                  `json:"obproxy_sys_password_sha1"`
	Backup                *OceanBaseBackupConfig  `json:"backup,omitempty"`
	Restore               *OceanBaseRestoreConfig `json:"restore,omitempty"`
}

type OceanBaseBackupConfig struct {
	Destination    string `json:"destination"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type OceanBaseRestoreConfig struct {
	Tenant           string                `json:"tenant"`
	BackupSource     string                `json:"backup_source"`
	ResourcePool     string                `json:"resource_pool"`
	UntilSCN         int64                 `json:"until_scn,omitempty"`
	TimeoutSeconds   int                   `json:"timeout_seconds,omitempty"`
	ValidationTables []OceanBaseTableCheck `json:"validation_tables"`
}

type OceanBaseTableCheck struct {
	Database         string `json:"database"`
	Table            string `json:"table"`
	ExpectedRowCount int64  `json:"expected_row_count"`
	ExpectedChecksum string `json:"expected_checksum"`
}

type flavorCommandRunner func(ctx context.Context, name string, args ...string) (string, error)
type flavorProcessStarter func(ctx context.Context, name string, args ...string) error

type flavorTaskHandler struct {
	flavor        string
	versionBinary string
	runner        flavorCommandRunner
}

// FlavorTaskExecutor registers isolated command builders and version detectors
// for supported 信创 flavors. Product lifecycle actions remain unavailable until
// the corresponding flavor executor is implemented.
type FlavorTaskExecutor struct {
	packageRoot string
	handlers    map[string]*flavorTaskHandler
	starter     flavorProcessStarter
}

func NewFlavorTaskExecutor() *FlavorTaskExecutor {
	return NewFlavorTaskExecutorWithPackageRootAndStarter(defaultLocalPackageRoot, commandOutputWithError, startCommand)
}

func startCommand(ctx context.Context, name string, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Observer and OBProxy remain running after the task returns.
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func commandOutputWithError(ctx context.Context, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(commandCtx, name, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func NewFlavorTaskExecutorWithPackageRoot(packageRoot string, runner flavorCommandRunner) *FlavorTaskExecutor {
	return NewFlavorTaskExecutorWithPackageRootAndStarter(packageRoot, runner, startCommand)
}

func NewFlavorTaskExecutorWithPackageRootAndStarter(packageRoot string, runner flavorCommandRunner, starter flavorProcessStarter) *FlavorTaskExecutor {
	if packageRoot == "" {
		packageRoot = defaultLocalPackageRoot
	}
	handlers := make(map[string]*flavorTaskHandler)
	for flavor, binary := range map[string]string{
		"oceanbase": "observer", "gaussdb-mysql": "gaussdb", "polardb-mysql": "polardb",
		"tdsql-mysql": "tdsql", "tidb": "tidb-server", "kingbase": "kingbase",
		"opengauss": "gaussdb", "highgo": "highgo", "gbase8a": "gccli",
		"shentong": "isql", "dm": "disql", "gbase8s": "onstat",
	} {
		handlers[flavor] = &flavorTaskHandler{flavor: flavor, versionBinary: binary, runner: runner}
	}
	return &FlavorTaskExecutor{packageRoot: packageRoot, handlers: handlers, starter: starter}
}

func (e *FlavorTaskExecutor) Execute(ctx context.Context, req FlavorTaskRequest) (*TaskResult, error) {
	if err := e.validateRequest(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	handler := e.handlers[normalizeFlavorTask(req.Flavor)]
	if handler == nil || handler.flavor != normalizeFlavorTask(req.Flavor) {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("flavor %q is not registered", req.Flavor)), nil
	}
	manifest, err := ValidateLocalPackageBundle(e.packageRoot, handler.flavor, req.Version)
	if err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}

	switch req.Operation {
	case "validate-package":
		return flavorTaskCompleted(req.TaskID, "local package bundle validated", map[string]interface{}{"flavor": handler.flavor, "version": req.Version}), nil
	case "version-detect":
		version, err := handler.detectVersion(ctx, req)
		if err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "flavor version detected", map[string]interface{}{"flavor": handler.flavor, "version": version}), nil
	case "deploy", "configure", "backup", "restore", "migrate", "upgrade", "monitor", "ha", "replication", "failover":
		if handler.flavor != "oceanbase" {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("operation %q is not executable for flavor %q", req.Operation, handler.flavor)), nil
		}
		return e.executeOceanBase(ctx, req, manifest)
	default:
		return flavorTaskFailure(req.TaskID, fmt.Errorf("operation %q is not executable for flavor %q", req.Operation, handler.flavor)), nil
	}
}

func (e *FlavorTaskExecutor) validateRequest(req FlavorTaskRequest) error {
	if strings.TrimSpace(req.TaskID) == "" || strings.TrimSpace(req.InstanceID) == "" {
		return fmt.Errorf("task_id and instance_id are required")
	}
	flavor := normalizeFlavorTask(req.Flavor)
	if flavor == "" || e.handlers[flavor] == nil {
		return fmt.Errorf("flavor %q is not registered", req.Flavor)
	}
	if strings.TrimSpace(req.Version) == "" || strings.TrimSpace(req.Operation) == "" {
		return fmt.Errorf("version and operation are required")
	}
	if req.TLS == nil {
		return fmt.Errorf("tls configuration is required")
	}
	expectedPath := filepath.Join(e.packageRoot, flavor, req.Version)
	if filepath.Clean(req.PackagePath) != expectedPath {
		return fmt.Errorf("package_path %q does not match flavor bundle %q", req.PackagePath, expectedPath)
	}
	return nil
}

func (h *flavorTaskHandler) versionCommand(req FlavorTaskRequest) (string, []string) {
	return filepath.Join(req.PackagePath, "bin", h.versionBinary), []string{"--version"}
}

func (h *flavorTaskHandler) detectVersion(ctx context.Context, req FlavorTaskRequest) (string, error) {
	if h.runner == nil {
		return "", fmt.Errorf("version detector is not configured")
	}
	binary, args := h.versionCommand(req)
	output, err := h.runner(ctx, binary, args...)
	if err != nil {
		return "", fmt.Errorf("detect %s version: %w", h.flavor, err)
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return "", fmt.Errorf("detect %s version returned empty output", h.flavor)
	}
	expectedVersion := strings.TrimPrefix(req.Version, "v")
	if !strings.Contains(output, expectedVersion) {
		return "", fmt.Errorf("detected %s version %q does not match requested version %q", h.flavor, output, req.Version)
	}
	return output, nil
}

func normalizeFlavorTask(flavor string) string {
	return strings.ToLower(strings.TrimSpace(flavor))
}

func flavorTaskFailure(taskID string, err error) *TaskResult {
	return &TaskResult{TaskID: taskID, Status: "failed", Progress: 0, Message: err.Error(), Timestamp: time.Now()}
}

func flavorTaskCompleted(taskID, message string, data map[string]interface{}) *TaskResult {
	return &TaskResult{TaskID: taskID, Status: "completed", Progress: 100, Message: message, Timestamp: time.Now(), Data: data}
}
