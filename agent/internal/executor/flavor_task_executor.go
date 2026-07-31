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
}

type flavorCommandRunner func(ctx context.Context, name string, args ...string) (string, error)

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
}

func NewFlavorTaskExecutor() *FlavorTaskExecutor {
	return NewFlavorTaskExecutorWithPackageRoot(defaultLocalPackageRoot, commandOutputWithError)
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
	return &FlavorTaskExecutor{packageRoot: packageRoot, handlers: handlers}
}

func (e *FlavorTaskExecutor) Execute(ctx context.Context, req FlavorTaskRequest) (*TaskResult, error) {
	if err := e.validateRequest(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	handler := e.handlers[normalizeFlavorTask(req.Flavor)]
	if handler == nil || handler.flavor != normalizeFlavorTask(req.Flavor) {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("flavor %q is not registered", req.Flavor)), nil
	}
	if _, err := ValidateLocalPackageBundle(e.packageRoot, handler.flavor, req.Version); err != nil {
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
