package executor

import (
	"context"
	"fmt"
	"os"
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
	TiDB        *TiDBConfig      `json:"tidb,omitempty"`
	Dameng      *DamengConfig    `json:"dameng,omitempty"`
	Kingbase    *KingbaseConfig  `json:"kingbase,omitempty"`
	OpenGauss   *OpenGaussConfig `json:"opengauss,omitempty"`
	GBase8s     *GBase8sConfig   `json:"gbase8s,omitempty"`
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
	ConfirmUninstall      bool                    `json:"confirm_uninstall,omitempty"`
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

// TiDBConfig contains the fixed single-host inputs for an offline TiUP deployment.
type TiDBConfig struct {
	ClusterName      string               `json:"cluster_name"`
	Address          string               `json:"address"`
	Architecture     string               `json:"architecture"`
	DeployUser       string               `json:"deploy_user"`
	RootPassword     string               `json:"root_password"`
	Parameters       map[string]string    `json:"parameters"`
	Backup           *TiDBBackupConfig    `json:"backup,omitempty"`
	Restore          *TiDBRestoreConfig   `json:"restore,omitempty"`
	Migration        *TiDBMigrationConfig `json:"migration,omitempty"`
	UpgradeVersion   string               `json:"upgrade_version,omitempty"`
	ConfirmUninstall bool                 `json:"confirm_uninstall,omitempty"`
}

type TiDBBackupConfig struct {
	Destination    string `json:"destination"`
	LogDestination string `json:"log_destination,omitempty"`
}

type TiDBRestoreConfig struct {
	BackupSource     string           `json:"backup_source"`
	LogBackupSource  string           `json:"log_backup_source,omitempty"`
	RestoredTS       string           `json:"restored_ts,omitempty"`
	ValidationTables []TiDBTableCheck `json:"validation_tables"`
}

type TiDBMigrationConfig struct {
	DataSourceDir string `json:"data_source_dir"`
	SortedKVDir   string `json:"sorted_kv_dir"`
}

type TiDBTableCheck struct {
	Database         string `json:"database"`
	Table            string `json:"table"`
	ExpectedRowCount int64  `json:"expected_row_count"`
	ExpectedChecksum string `json:"expected_checksum"`
}

// DamengConfig contains the fixed single-instance inputs for an offline DM9 deployment.
type DamengConfig struct {
	Address             string                 `json:"address"`
	Port                int                    `json:"port"`
	InstallDir          string                 `json:"install_dir"`
	DataDir             string                 `json:"data_dir"`
	SysdbaPassword      string                 `json:"sysdba_password"`
	ApplicationUser     string                 `json:"application_user"`
	ApplicationPassword string                 `json:"application_password"`
	Parameters          map[string]string      `json:"parameters"`
	Backup              *DamengBackupConfig    `json:"backup,omitempty"`
	Restore             *DamengRestoreConfig   `json:"restore,omitempty"`
	Migration           *DamengMigrationConfig `json:"migration,omitempty"`
	ConfirmUninstall    bool                   `json:"confirm_uninstall,omitempty"`
}

type DamengBackupConfig struct {
	Destination string `json:"destination"`
	Offline     bool   `json:"offline,omitempty"`
}

type DamengRestoreConfig struct {
	BackupSource  string `json:"backup_source"`
	ArchiveSource string `json:"archive_source,omitempty"`
}

type DamengMigrationConfig struct {
	DumpFile string `json:"dump_file"`
}

// KingbaseConfig contains the constrained inputs for a KES V9R1C10
// single-node deployment. Installation and data directories stay below the
// Kingbase-owned root and credentials are passed through the input runner.
type KingbaseConfig struct {
	Address             string                   `json:"address"`
	Port                int                      `json:"port"`
	InstallDir          string                   `json:"install_dir"`
	DataDir             string                   `json:"data_dir"`
	SuperuserPassword   string                   `json:"superuser_password"`
	ApplicationUser     string                   `json:"application_user"`
	ApplicationPassword string                   `json:"application_password"`
	Parameters          map[string]string        `json:"parameters"`
	TLSCIDR             string                   `json:"tls_cidr,omitempty"`
	Backup              *KingbaseBackupConfig    `json:"backup,omitempty"`
	Restore             *KingbaseRestoreConfig   `json:"restore,omitempty"`
	Migration           *KingbaseMigrationConfig `json:"migration,omitempty"`
}

type KingbaseBackupConfig struct {
	Destination string `json:"destination"`
}

type KingbaseRestoreConfig struct {
	BackupSource   string `json:"backup_source"`
	TargetDatabase string `json:"target_database"`
	RecoveryTarget string `json:"recovery_target,omitempty"`
}

type KingbaseMigrationConfig struct {
	SourceDatabase string `json:"source_database"`
	TargetDatabase string `json:"target_database"`
	DumpFile       string `json:"dump_file"`
}

// OpenGaussConfig contains the constrained inputs for an offline openGauss
// Lite single-node deployment.
type OpenGaussConfig struct {
	Address             string                    `json:"address"`
	Port                int                       `json:"port"`
	InstallDir          string                    `json:"install_dir"`
	DataDir             string                    `json:"data_dir"`
	AdminPassword       string                    `json:"admin_password"`
	ApplicationUser     string                    `json:"application_user"`
	ApplicationPassword string                    `json:"application_password"`
	Parameters          map[string]string         `json:"parameters"`
	Backup              *OpenGaussBackupConfig    `json:"backup,omitempty"`
	Restore             *OpenGaussRestoreConfig   `json:"restore,omitempty"`
	Migration           *OpenGaussMigrationConfig `json:"migration,omitempty"`
	ConfirmUninstall    bool                      `json:"confirm_uninstall,omitempty"`
}

type OpenGaussBackupConfig struct {
	Destination string `json:"destination"`
}

type OpenGaussRestoreConfig struct {
	BackupSource   string `json:"backup_source"`
	RecoveryTarget string `json:"recovery_target,omitempty"`
}

type OpenGaussMigrationConfig struct {
	DumpFile string `json:"dump_file"`
}

// GBase8sConfig contains the constrained inputs for documented GBase 8s
// single-node lifecycle workflows.
type GBase8sConfig struct {
	InstallDir           string                  `json:"install_dir"`
	ApplicationUser      string                  `json:"application_user"`
	ApplicationPassword  string                  `json:"application_password"`
	PersistentParameters map[string]string       `json:"persistent_parameters"`
	MemoryParameters     map[string]string       `json:"memory_parameters"`
	ConfirmUninstall     bool                    `json:"confirm_uninstall,omitempty"`
	Backup               *GBase8sBackupConfig    `json:"backup,omitempty"`
	Restore              *GBase8sRestoreConfig   `json:"restore,omitempty"`
	Migration            *GBase8sMigrationConfig `json:"migration,omitempty"`
}

// GBase8sBackupConfig keeps ontape's database and logical-log media separate.
type GBase8sBackupConfig struct {
	TapeDirectory       string `json:"tape_directory"`
	LogicalLogDirectory string `json:"logical_log_directory"`
}

// GBase8sRestoreConfig selects the isolated ontape media and whether to replay
// the backed-up logical logs after the archive restore.
type GBase8sRestoreConfig struct {
	TapeDirectory       string `json:"tape_directory"`
	LogicalLogDirectory string `json:"logical_log_directory"`
	ReplayLogicalLogs   bool   `json:"replay_logical_logs"`
}

// GBase8sMigrationConfig constrains dbexport and dbimport to prepared local
// directories and SQL identifiers.
type GBase8sMigrationConfig struct {
	SourceDatabase  string `json:"source_database"`
	TargetDatabase  string `json:"target_database"`
	ExportDirectory string `json:"export_directory"`
	ImportDirectory string `json:"import_directory"`
}

type flavorCommandRunner func(ctx context.Context, name string, args ...string) (string, error)
type flavorProcessStarter func(ctx context.Context, name string, args ...string) error
type flavorCommandInputRunner func(ctx context.Context, input, name string, args ...string) (string, error)

type flavorTaskHandler struct {
	flavor        string
	versionBinary string
	runner        flavorCommandRunner
}

// FlavorTaskExecutor registers isolated command builders and version detectors
// for supported 信创 flavors. Product lifecycle actions remain unavailable until
// the corresponding flavor executor is implemented.
type FlavorTaskExecutor struct {
	packageRoot     string
	handlers        map[string]*flavorTaskHandler
	starter         flavorProcessStarter
	inputRunner     flavorCommandInputRunner
	readFile        func(string) ([]byte, error)
	removeAll       func(string) error
	tidbControlRoot string
}

func NewFlavorTaskExecutor() *FlavorTaskExecutor {
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(defaultLocalPackageRoot, commandOutputWithError, startCommand)
	executor.inputRunner = commandOutputWithInput
	return executor
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

func commandOutputWithInput(ctx context.Context, input, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(commandCtx, name, args...)
	command.Stdin = strings.NewReader(input)
	output, err := command.CombinedOutput()
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
	return &FlavorTaskExecutor{packageRoot: packageRoot, handlers: handlers, starter: starter, inputRunner: commandOutputWithInput, readFile: os.ReadFile, removeAll: os.RemoveAll, tidbControlRoot: tidbControlRoot}
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
	case "deploy", "configure", "backup", "restore", "migrate", "upgrade", "monitor", "ha", "replication", "failover", "scale", "teardown":
		switch handler.flavor {
		case "oceanbase":
			return e.executeOceanBase(ctx, req, manifest)
		case "tidb":
			return e.executeTiDB(ctx, req, manifest)
		case "dm":
			if req.Operation != "deploy" && req.Operation != "configure" && req.Operation != "backup" && req.Operation != "restore" && req.Operation != "migrate" && req.Operation != "monitor" && req.Operation != "teardown" && req.Operation != "ha" && req.Operation != "replication" && req.Operation != "failover" && req.Operation != "scale" {
				return flavorTaskFailure(req.TaskID, fmt.Errorf("operation %q is not executable for flavor %q", req.Operation, handler.flavor)), nil
			}
			return e.executeDameng(ctx, req, manifest)
		case "kingbase":
			return e.executeKingbase(ctx, req, manifest)
		case "opengauss":
			return e.executeOpenGauss(ctx, req, manifest)
		case "gbase8s":
			return e.executeGBase8s(ctx, req, manifest)
		default:
			return flavorTaskFailure(req.TaskID, fmt.Errorf("operation %q is not executable for flavor %q", req.Operation, handler.flavor)), nil
		}
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
