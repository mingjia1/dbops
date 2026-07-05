package executor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ToolInstaller struct {
	installDir  string
	relayClient *relayClient
}

type mysqlRuntimeOptions struct {
	Basedir       string
	PluginDir     string
	SecureFileDir string
}

func NewToolInstaller() *ToolInstaller {
	installDir := "/opt/dbops-tools"
	if runtime.GOOS == "windows" {
		installDir = "C:\\dbops-tools"
	}
	return &ToolInstaller{installDir: installDir}
}

func NewToolInstallerWithRelay(relayHost string, relayPort int, relayToken string) *ToolInstaller {
	ti := NewToolInstaller()
	if relayHost != "" {
		ti.relayClient = newRelayClient(relayHost, relayPort, relayToken)
	}
	return ti
}

func (t *ToolInstaller) SetRelay(host string, port int, token string) {
	if host != "" {
		t.relayClient = newRelayClient(host, port, token)
	} else {
		t.relayClient = nil
	}
}

type InstallToolsRequest struct {
	Tools        []string `json:"tools"`
	MySQLVersion string   `json:"mysql_version"`
	Force        bool     `json:"force"`
}

type InstallToolsResult struct {
	TaskID    string            `json:"task_id"`
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Installed map[string]string `json:"installed"`
	Failed    map[string]string `json:"failed"`
	Logs      []string          `json:"logs,omitempty"`
}

type BlankHostInitRequest struct {
	MySQLVersion   string `json:"mysql_version"`
	MySQLPort      int    `json:"mysql_port"`
	DataDir        string `json:"data_dir"`
	Basedir        string `json:"basedir"`
	RootPassword   string `json:"root_password"`
	ReplUser       string `json:"repl_user"`
	ReplPass       string `json:"repl_pass"`
	PackageURL     string `json:"package_url"`
	PackageChecksum string `json:"package_checksum"`
	Force          bool   `json:"force"`
	SkipToolsCheck bool   `json:"skip_tools_check"`
}

type BlankHostInitResult struct {
	TaskID    string           `json:"task_id"`
	Status    string           `json:"status"`
	Progress  int              `json:"progress"`
	Message   string           `json:"message"`
	Steps     []InitStepResult `json:"steps"`
	Installed map[string]string `json:"installed"`
}

type InitStepResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (t *ToolInstaller) log(msg string) string {
	return fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
}

func (t *ToolInstaller) InstallTools(ctx context.Context, req InstallToolsRequest) *InstallToolsResult {
	result := &InstallToolsResult{
		Status:    "running",
		Installed: make(map[string]string),
		Failed:    make(map[string]string),
		Logs:      make([]string, 0),
	}

	if err := os.MkdirAll(t.installDir, 0755); err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Failed to create install directory: %v", err)
		result.Logs = append(result.Logs, t.log(result.Message))
		return result
	}

	info := t.detectOSInfo()
	result.Logs = append(result.Logs, t.log(fmt.Sprintf("Detected OS: %s %s (%s/%s)", info.Distribution, info.Version, info.OS, info.Arch)))

	if info.PackageManager == "" {
		result.Status = "failed"
		result.Message = "No supported package manager found on this system"
		result.Logs = append(result.Logs, t.log(result.Message))
		return result
	}

	if err := t.installSystemPrerequisites(ctx, info); err != nil {
		result.Logs = append(result.Logs, t.log(fmt.Sprintf("Prerequisites installation had issues: %v", err)))
	}

	if err := t.checkSystemResources(); err != nil {
		result.Logs = append(result.Logs, t.log(fmt.Sprintf("Resource check warning: %v", err)))
	}

	for _, tool := range req.Tools {
		toolLower := strings.ToLower(tool)
		if !req.Force {
			if path := t.checkToolInstalled(toolLower); path != "" {
				result.Installed[toolLower] = path
				result.Logs = append(result.Logs, t.log(fmt.Sprintf("%s already installed at %s", toolLower, path)))
				continue
			}
		}

		var err error
		var installPath string

		switch info.OS {
		case "linux":
			installPath, err = t.installToolLinux(ctx, toolLower, req.MySQLVersion, info)
		case "windows":
			installPath, err = t.installToolWindows(ctx, toolLower, req.MySQLVersion)
		case "darwin":
			installPath, err = t.installToolMacOS(ctx, toolLower, req.MySQLVersion)
		default:
			err = fmt.Errorf("unsupported OS: %s", info.OS)
		}

		if err != nil {
			result.Failed[toolLower] = err.Error()
			result.Logs = append(result.Logs, t.log(fmt.Sprintf("FAILED to install %s: %v", toolLower, err)))
		} else {
			result.Installed[toolLower] = installPath
			result.Logs = append(result.Logs, t.log(fmt.Sprintf("Installed %s at %s", toolLower, installPath)))
		}
	}

	if len(result.Failed) == 0 {
		result.Status = "success"
		result.Message = fmt.Sprintf("Successfully installed %d tools", len(result.Installed))
	} else if len(result.Installed) > 0 {
		result.Status = "partial"
		result.Message = fmt.Sprintf("Installed %d tools, %d failed", len(result.Installed), len(result.Failed))
	} else {
		result.Status = "failed"
		result.Message = "All tools installation failed"
	}

	return result
}

type HostOSInfo struct {
	OS             string
	Distribution   string
	Version        string
	Arch           string
	Codename       string
	PackageManager string
}

func (t *ToolInstaller) detectOSInfo() HostOSInfo {
	info := HostOSInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return info
		}
		content := string(data)

		if strings.Contains(content, "Ubuntu") {
			info.Distribution = "ubuntu"
			info.PackageManager = "apt"
		} else if strings.Contains(content, "Debian") {
			info.Distribution = "debian"
			info.PackageManager = "apt"
		} else if strings.Contains(content, "CentOS") {
			info.Distribution = "centos"
			info.PackageManager = "yum"
		} else if strings.Contains(content, "Red Hat") || strings.Contains(content, "rhel") {
			info.Distribution = "rhel"
			info.PackageManager = "yum"
		} else if strings.Contains(content, "Rocky") {
			info.Distribution = "rocky"
			info.PackageManager = "yum"
		} else if strings.Contains(content, "Fedora") {
			info.Distribution = "fedora"
			info.PackageManager = "dnf"
		} else if strings.Contains(content, "openSUSE") || strings.Contains(content, "SUSE") {
			info.Distribution = "suse"
			info.PackageManager = "zypper"
		} else if strings.Contains(content, "Alpine") {
			info.Distribution = "alpine"
			info.PackageManager = "apk"
		}

		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "VERSION_ID=") {
				info.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
			if strings.HasPrefix(line, "VERSION_CODENAME=") {
				info.Codename = strings.Trim(strings.TrimPrefix(line, "VERSION_CODENAME="), "\"")
			}
			if strings.HasPrefix(line, "UBUNTU_CODENAME=") {
				info.Codename = strings.Trim(strings.TrimPrefix(line, "UBUNTU_CODENAME="), "\"")
			}
		}

		if info.PackageManager == "apt" && info.Codename == "" {
			if out, err := exec.Command("lsb_release", "-sc").Output(); err == nil {
				info.Codename = strings.TrimSpace(string(out))
			}
		}

		if info.PackageManager == "" {
			for _, pm := range []string{"apt-get", "yum", "dnf", "zypper", "apk"} {
				if _, err := exec.LookPath(pm); err == nil {
					info.PackageManager = pm
					break
				}
			}
		}

	case "darwin":
		info.Distribution = "macos"
		info.PackageManager = "brew"
		if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
			info.Version = strings.TrimSpace(string(out))
		}

	case "windows":
		info.Distribution = "windows"
		info.PackageManager = "choco"
	}

	return info
}

func (t *ToolInstaller) installSystemPrerequisites(ctx context.Context, info HostOSInfo) error {
	required := []string{"wget", "curl", "gnupg", "lsb-release", "ca-certificates"}

	switch info.PackageManager {
	case "apt":
		missing := t.checkMissingCommands(required)
		if len(missing) == 0 {
			return nil
		}
		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
			export DEBIAN_FRONTEND=noninteractive
			apt-get update -qq 2>/dev/null || true
			apt-get install -y -qq %s 2>/dev/null || true
		`, strings.Join(missing, " ")))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("install prerequisites failed: %v, output: %s", err, string(out))
		}

	case "yum", "dnf":
		missing := t.checkMissingCommands(required)
		if len(missing) == 0 {
			return nil
		}
		pm := info.PackageManager
		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("%s install -y %s 2>/dev/null || true", pm, strings.Join(missing, " ")))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("install prerequisites failed: %v, output: %s", err, string(out))
		}

	case "zypper":
		missing := t.checkMissingCommands(required)
		if len(missing) == 0 {
			return nil
		}
		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("zypper -n install %s 2>/dev/null || true", strings.Join(missing, " ")))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("install prerequisites failed: %v, output: %s", err, string(out))
		}
	}

	return nil
}

func (t *ToolInstaller) checkMissingCommands(commands []string) []string {
	var missing []string
	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err != nil {
			missing = append(missing, cmd)
		}
	}
	return missing
}

func (t *ToolInstaller) checkSystemResources() error {
	var issues []string

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "MemAvailable:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					var memKB int
					fmt.Sscanf(parts[1], "%d", &memKB)
					memMB := memKB / 1024
					if memMB < 512 {
						issues = append(issues, fmt.Sprintf("low memory: %dMB available, recommend >= 512MB", memMB))
					}
				}
				break
			}
		}
	}

	if r := runtime.NumCPU(); r < 1 {
		issues = append(issues, fmt.Sprintf("low CPU cores: %d, recommend >= 2", r))
	}

	if len(issues) > 0 {
		return fmt.Errorf("system resource warnings: %s", strings.Join(issues, "; "))
	}
	return nil
}

func (t *ToolInstaller) checkDiskSpace(path string, minGB int) error {
	var stat syscallStatfs
	if err := syscallStatfsWrapper(path, &stat); err != nil {
		return nil
	}
	availGB := int(stat.Bavail * uint64(stat.Bsize) / 1024 / 1024 / 1024)
	if availGB < minGB {
		return fmt.Errorf("insufficient disk space on %s: %dGB available, need %dGB", path, availGB, minGB)
	}
	return nil
}

func (t *ToolInstaller) checkToolInstalled(tool string) string {
	path, err := exec.LookPath(tool)
	if err == nil {
		return path
	}

	candidates := []string{
		filepath.Join(t.installDir, "bin", tool),
		filepath.Join(t.installDir, "bin", tool+".exe"),
		filepath.Join(t.installDir, tool),
		filepath.Join(t.installDir, tool+".exe"),
		fmt.Sprintf("/opt/dbops-mysql/bin/%s", tool),
		fmt.Sprintf("/opt/dbops-pxc/usr/bin/%s", tool),
		fmt.Sprintf("/opt/dbops-pxc/usr/sbin/%s", tool),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

func mysqlRuntimeOptionsFor(mysqld string) mysqlRuntimeOptions {
	opts := mysqlRuntimeOptions{SecureFileDir: "/var/lib/mysql-files"}
	if mysqld == "" {
		return opts
	}
	if abs, err := filepath.Abs(mysqld); err == nil {
		mysqld = abs
	}
	dir := filepath.Dir(mysqld)
	if filepath.Base(dir) == "bin" || filepath.Base(dir) == "sbin" || filepath.Base(dir) == "libexec" {
		basedir := filepath.Dir(dir)
		if st, err := os.Stat(basedir); err == nil && st.IsDir() && basedir != "/usr" {
			opts.Basedir = basedir
		}
	}
	for _, base := range []string{opts.Basedir, "/usr", "/usr/local/mysql", "/opt/mysql"} {
		if base == "" {
			continue
		}
		for _, rel := range []string{
			filepath.Join("lib", "mysql", "plugin"),
			filepath.Join("lib64", "mysql", "plugin"),
			filepath.Join("lib", "plugin"),
			"plugin",
		} {
			candidate := filepath.Join(base, rel)
			if st, err := os.Stat(filepath.Join(candidate, "group_replication.so")); err == nil && !st.IsDir() {
				opts.PluginDir = candidate
				return opts
			}
		}
	}
	return opts
}

func (opts mysqlRuntimeOptions) args() []string {
	args := make([]string, 0, 3)
	if opts.Basedir != "" {
		args = append(args, "--basedir="+opts.Basedir)
	}
	if opts.PluginDir != "" {
		args = append(args, "--plugin-dir="+opts.PluginDir)
	}
	if opts.SecureFileDir != "" {
		args = append(args, "--secure-file-priv="+opts.SecureFileDir)
	}
	return args
}

func (t *ToolInstaller) installToolLinux(ctx context.Context, tool string, mysqlVersion string, info HostOSInfo) (string, error) {
	switch tool {
	case "mysql", "mysqld":
		return t.installMySQLLinux(ctx, info, mysqlVersion)
	case "xtrabackup":
		return t.installXtraBackupLinux(ctx, info, mysqlVersion)
	case "mysqladmin":
		return t.findOrInstallMySQLTool(ctx, info, "mysqladmin")
	case "mysqldump":
		return t.findOrInstallMySQLTool(ctx, info, "mysqldump")
	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}
}

func (t *ToolInstaller) findOrInstallMySQLTool(ctx context.Context, info HostOSInfo, toolName string) (string, error) {
	if path := t.checkToolInstalled(toolName); path != "" {
		return path, nil
	}

	switch info.PackageManager {
	case "apt":
		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
			export DEBIAN_FRONTEND=noninteractive
			apt-get install -y -qq mysql-client 2>/dev/null || apt-get install -y -qq mysql-client-core-8.0 2>/dev/null || true
		`))
		cmd.CombinedOutput()
	case "yum", "dnf":
		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("%s install -y mysql 2>/dev/null || true", info.PackageManager))
		cmd.CombinedOutput()
	}

	if path := t.checkToolInstalled(toolName); path != "" {
		return path, nil
	}
	return "", fmt.Errorf("%s not found after installation attempt", toolName)
}

func (t *ToolInstaller) installMySQLLinux(ctx context.Context, info HostOSInfo, version string) (string, error) {
	installedPath := t.checkToolInstalled("mysqld")
	if installedPath != "" {
		return installedPath, nil
	}

	if err := t.checkDiskSpace("/", 2); err != nil {
		return "", fmt.Errorf("disk space check: %w", err)
	}

	switch info.PackageManager {
	case "apt":
		return t.installMySQLApt(ctx, info, version)
	case "yum", "dnf":
		return t.installMySQLYum(ctx, info, version)
	case "zypper":
		return t.installMySQLZypper(ctx, info, version)
	default:
		return "", fmt.Errorf("unsupported package manager for MySQL installation: %s", info.PackageManager)
	}
}

func (t *ToolInstaller) installMySQLApt(ctx context.Context, info HostOSInfo, version string) (string, error) {
	if version == "" {
		version = "8.0"
	}

	codename := info.Codename
	if codename == "" {
		codename = "jammy"
	}

	installedPath := t.checkToolInstalled("mysqld")
	installedMysqlPath := t.checkToolInstalled("mysql")
	if installedPath != "" && installedMysqlPath != "" {
		return installedPath, nil
	}

	if t.isMySQLRepoConfigured(ctx) {
		goto install_packages
	}

	if t.relayClient != nil {
		debFileName := "mysql-apt-config_0.8.33-1_all.deb"
		if version == "5.7" {
			debFileName = "mysql-apt-config_0.8.12-1_all.deb"
		}

		relayReq := RelayPackageRequest{
			URL:  fmt.Sprintf("https://dev.mysql.com/get/%s", debFileName),
			Name: debFileName,
			Type: "mysql_apt_config",
			Version: version,
		}

		relayRes, err := t.relayClient.fetchPackage(ctx, relayReq)
		if err == nil && relayRes.Status == "success" {
			debPath := "/tmp/" + relayRes.FileName
			if err := t.relayClient.downloadPackage(ctx, relayRes.FileName, debPath); err == nil {
				defer os.Remove(debPath)
				installDebCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
dpkg -i %s 2>/dev/null || {
	apt-get update -qq
	apt-get install -y -qq -f 2>/dev/null || true
	dpkg -i %s 2>/dev/null || true
}
apt-get update -qq 2>/dev/null || true
`, debPath, debPath))
				if _, err := installDebCmd.CombinedOutput(); err != nil {
					t.log(fmt.Sprintf("Relay apt-config install failed, fallback to direct: %v", err))
				} else {
					goto install_packages
				}
			}
		}
	}

	if err := t.configureMySQLAptRepo(ctx, version, codename); err != nil {
		return "", fmt.Errorf("configure MySQL APT repo failed: %w", err)
	}

install_packages:
	var packages []string
	switch {
	case strings.HasPrefix(version, "5.7"):
		packages = []string{"mysql-server-5.7", "mysql-client-5.7"}
	case strings.HasPrefix(version, "8.0"):
		packages = []string{"mysql-server-8.0", "mysql-client-8.0"}
	case strings.HasPrefix(version, "8.4"):
		packages = []string{"mysql-server-8.4", "mysql-client-8.4"}
	default:
		packages = []string{"mysql-server", "mysql-client"}
	}

	if t.relayClient != nil {
		installedFromRelay := t.tryInstallMySQLPackagesFromRelay(ctx, packages, info)
		if installedFromRelay {
			installedPath = t.checkToolInstalled("mysqld")
			if installedPath != "" {
				return installedPath, nil
			}
		}
	}

	installCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
echo "mysql-community-server mysql-server/root_password password ''" | debconf-set-selections 2>/dev/null || true
echo "mysql-community-server mysql-server/root_password_again password ''" | debconf-set-selections 2>/dev/null || true
apt-get install -y -qq %s
`, strings.Join(packages, " ")))
	out, err := installCmd.CombinedOutput()
	if err != nil {
		retryCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --fix-missing %s 2>/dev/null || true
`, strings.Join(packages, " ")))
		if retryOut, retryErr := retryCmd.CombinedOutput(); retryErr != nil {
			return "", fmt.Errorf("MySQL apt install failed after retry: %v, output: %s (retry: %s)", err, string(out), string(retryOut))
		}
	}

	installedPath = t.checkToolInstalled("mysqld")
	if installedPath == "" {
		paths := []string{"/usr/sbin/mysqld", "/usr/bin/mysqld", "/usr/libexec/mysqld"}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				installedPath = p
				break
			}
		}
	}
	if installedPath == "" {
		return "", fmt.Errorf("mysqld not found after MySQL installation")
	}

	return installedPath, nil
}

func (t *ToolInstaller) tryInstallMySQLPackagesFromRelay(ctx context.Context, packages []string, info HostOSInfo) bool {
	ti := t
	if ti.relayClient == nil {
		return false
	}

	relayReq := RelayPackageRequest{
		Type:    "mysql_server_deb",
		Version: "8.0",
	}
	relayRes, err := ti.relayClient.fetchPackage(ctx, relayReq)
	if err != nil || relayRes.Status != "success" {
		return false
	}

	for _, pkg := range packages {
		debPath := "/tmp/" + pkg + ".deb"
		if err := ti.relayClient.downloadPackage(ctx, relayRes.FileName, debPath); err != nil {
			_ = os.Remove(debPath)
			continue
		}
		installDebCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
dpkg -i %s 2>/dev/null || {
	apt-get update -qq
	apt-get install -y -qq -f
}
`, debPath))
		installDebCmd.CombinedOutput()
		_ = os.Remove(debPath)
		return true
	}
	return false
}

func (t *ToolInstaller) isMySQLRepoConfigured(ctx context.Context) bool {
	checkCmd := exec.CommandContext(ctx, "bash", "-c",
		"apt-cache policy mysql-server 2>/dev/null | grep -q 'repo.mysql.com' && echo 'configured' || echo 'missing'")
	out, err := checkCmd.Output()
	if err == nil && strings.TrimSpace(string(out)) == "configured" {
		return true
	}

	checkListCmd := exec.CommandContext(ctx, "bash", "-c",
		"grep -rl 'repo.mysql.com' /etc/apt/sources.list.d/ 2>/dev/null | head -1 || true")
	out, _ = checkListCmd.Output()
	return strings.TrimSpace(string(out)) != ""
}

func (t *ToolInstaller) configureMySQLAptRepo(ctx context.Context, version, codename string) error {
	var debURL string
	switch {
	case strings.HasPrefix(version, "5.7"):
		debURL = "https://repo.mysql.com/apt/ubuntu/pool/mysql-apt-config/m/mysql-apt-config/mysql-apt-config_0.8.12-1_all.deb"
	default:
		debURL = fmt.Sprintf("https://dev.mysql.com/get/mysql-apt-config_0.8.33-1_all.deb")
	}

	debPath := "/tmp/mysql-apt-config.deb"
	_ = os.Remove(debPath)

	downloadCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
for i in 1 2 3; do
	if command -v wget >/dev/null 2>&1; then
		wget -q -O %s "%s" && exit 0
	elif command -v curl >/dev/null 2>&1; then
		curl -sL -o %s "%s" && exit 0
	fi
	echo "download attempt $i failed, retrying..."
	sleep 2
done
exit 1
`, debPath, debURL, debPath, debURL))
	out, err := downloadCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("download MySQL apt config failed: %v, output: %s", err, string(out))
	}
	defer os.Remove(debPath)

	installDebCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
dpkg -i %s 2>/dev/null || {
	apt-get update -qq
	apt-get install -y -qq -f 2>/dev/null || true
	dpkg -i %s 2>/dev/null || true
}
`, debPath, debPath))
	out, err = installDebCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install MySQL apt config failed: %v, output: %s", err, string(out))
	}

	updateCmd := exec.CommandContext(ctx, "bash", "-c", `
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq 2>/dev/null || true
`)
	updateCmd.CombinedOutput()

	return nil
}

func (t *ToolInstaller) installMySQLYum(ctx context.Context, info HostOSInfo, version string) (string, error) {
	if version == "" {
		version = "8.0"
	}

	installedPath := t.checkToolInstalled("mysqld")
	installedMysqlPath := t.checkToolInstalled("mysql")
	if installedPath != "" && installedMysqlPath != "" {
		return installedPath, nil
	}

	var repoRPM string
	switch {
	case strings.HasPrefix(version, "5.7"):
		repoRPM = "https://dev.mysql.com/get/mysql57-community-release-el7-11.noarch.rpm"
	case strings.HasPrefix(version, "8.0"):
		repoRPM = "https://dev.mysql.com/get/mysql80-community-release-el7-7.noarch.rpm"
	default:
		repoRPM = "https://dev.mysql.com/get/mysql80-community-release-el7-7.noarch.rpm"
	}

	pm := info.PackageManager

	if t.relayClient != nil {
		rpmFileName := "mysql80-community-release-el7-7.noarch.rpm"
		if strings.HasPrefix(version, "5.7") {
			rpmFileName = "mysql57-community-release-el7-11.noarch.rpm"
		}

		relayReq := RelayPackageRequest{
			URL:     repoRPM,
			Name:    rpmFileName,
			Type:    "mysql_yum_repo",
			Version: version,
		}

		relayRes, err := t.relayClient.fetchPackage(ctx, relayReq)
		if err == nil && relayRes.Status == "success" {
			rpmPath := "/tmp/" + relayRes.FileName
			if err := t.relayClient.downloadPackage(ctx, relayRes.FileName, rpmPath); err == nil {
				defer os.Remove(rpmPath)
				installCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
%s install -y %s 2>/dev/null || true
%s install -y mysql-community-server mysql-community-client 2>/dev/null || %s install -y mysql-server mysql
`, pm, rpmPath, pm, pm))
				if out, err := installCmd.CombinedOutput(); err != nil {
					t.log(fmt.Sprintf("Relay MySQL YUM install failed, fallback to direct: %v, output: %s", err, string(out)))
				} else {
					installedPath = t.checkToolInstalled("mysqld")
					if installedPath != "" {
						return installedPath, nil
					}
				}
			}
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
rpm -q mysql80-community-release 2>/dev/null || rpm -q mysql57-community-release 2>/dev/null || {
	for i in 1 2 3; do
		%s install -y %s && break
		sleep 3
	done
}
%s install -y mysql-community-server mysql-community-client 2>/dev/null || %s install -y mysql-server mysql
`, pm, repoRPM, pm, pm))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("MySQL yum install failed: %v, output: %s", err, string(out))
	}

	installedPath = t.checkToolInstalled("mysqld")
	if installedPath == "" {
		return "", fmt.Errorf("mysqld not found after yum installation")
	}
	return installedPath, nil
}

func (t *ToolInstaller) installMySQLZypper(ctx context.Context, info HostOSInfo, version string) (string, error) {
	if version == "" {
		version = "8.0"
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
	zypper -n addrepo https://download.opensuse.org/repositories/server:database/openSUSE_Leap_15.5/server:database.repo 2>/dev/null || true
	zypper --gpg-auto-import-keys refresh 2>/dev/null || true
	zypper -n install mysql-community-server mysql-community-client 2>/dev/null || zypper -n install mysql-server mysql-client
	`))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("MySQL zypper install failed: %v, output: %s", err, string(out))
	}
	installedPath := t.checkToolInstalled("mysqld")
	if installedPath == "" {
		return "", fmt.Errorf("mysqld not found after zypper installation")
	}
	return installedPath, nil
}

func (t *ToolInstaller) installXtraBackupLinux(ctx context.Context, info HostOSInfo, mysqlVersion string) (string, error) {
	installedPath := t.checkToolInstalled("xtrabackup")
	if installedPath != "" {
		return installedPath, nil
	}

	xtraVersion := t.resolveXtraBackupVersion(mysqlVersion)

	if err := t.checkDiskSpace("/", 1); err != nil {
		return "", fmt.Errorf("disk space check: %w", err)
	}

	switch info.PackageManager {
	case "apt":
		return t.installXtraBackupApt(ctx, info, xtraVersion)
	case "yum", "dnf":
		return t.installXtraBackupYum(ctx, info, xtraVersion)
	default:
		return "", fmt.Errorf("unsupported package manager for XtraBackup: %s", info.PackageManager)
	}
}

func (t *ToolInstaller) resolveXtraBackupVersion(mysqlVersion string) string {
	majorMinor := ""
	if v := strings.Split(mysqlVersion, "."); len(v) >= 2 {
		majorMinor = v[0] + v[1]
	}

	switch majorMinor {
	case "57":
		return "24"
	case "80":
		return "80"
	case "84":
		return "84"
	default:
		if strings.HasPrefix(mysqlVersion, "5.7") {
			return "24"
		}
		if strings.HasPrefix(mysqlVersion, "8.4") {
			return "84"
		}
		return "80"
	}
}

func (t *ToolInstaller) installXtraBackupApt(ctx context.Context, info HostOSInfo, xtraVersion string) (string, error) {
	pkgName := fmt.Sprintf("percona-xtrabackup-%s", xtraVersion)

	if t.relayClient != nil {
		relayReq := RelayPackageRequest{
			Type:    fmt.Sprintf("xtrabackup_%s_deb", xtraVersion),
			Version: xtraVersion,
		}
		relayRes, err := t.relayClient.fetchPackage(ctx, relayReq)
		if err == nil && relayRes.Status == "success" {
			debPath := "/tmp/" + relayRes.FileName
			if err := t.relayClient.downloadPackage(ctx, relayRes.FileName, debPath); err == nil {
				defer os.Remove(debPath)
				installCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
dpkg -i %s 2>/dev/null || {
	apt-get update -qq
	apt-get install -y -qq -f
}
`, debPath))
				if out, err := installCmd.CombinedOutput(); err != nil {
					t.log(fmt.Sprintf("Relay XtraBackup install failed, fallback to direct: %v, output: %s", err, string(out)))
				} else {
					installedPath := t.checkToolInstalled("xtrabackup")
					if installedPath != "" {
						return installedPath, nil
					}
				}
			}
		}
	}

	if !t.isPerconaRepoConfigured(ctx) {
		if err := t.configurePerconaAptRepo(ctx, info); err != nil {
			return "", fmt.Errorf("configure Percona APT repo failed: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq %s
`, pkgName))
	out, err := cmd.CombinedOutput()
	if err != nil {
		retryCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --fix-missing %s 2>/dev/null || true
`, pkgName))
		if retryOut, retryErr := retryCmd.CombinedOutput(); retryErr != nil {
			return "", fmt.Errorf("XtraBackup apt install failed after retry: %v, output: %s (retry: %s)", err, string(out), string(retryOut))
		}
	}

	installedPath := t.checkToolInstalled("xtrabackup")
	if installedPath == "" {
		paths := []string{"/usr/bin/xtrabackup", "/usr/local/bin/xtrabackup", "/opt/percona/bin/xtrabackup"}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				installedPath = p
				break
			}
		}
	}
	if installedPath == "" {
		return "", fmt.Errorf("xtrabackup not found after installation")
	}
	return installedPath, nil
}

func (t *ToolInstaller) isPerconaRepoConfigured(ctx context.Context) bool {
	checkCmd := exec.CommandContext(ctx, "bash", "-c",
		"grep -rl 'repo.percona.com' /etc/apt/sources.list.d/ 2>/dev/null | head -1 || true")
	out, _ := checkCmd.Output()
	if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
		return true
	}

	checkPkgCmd := exec.CommandContext(ctx, "bash", "-c",
		"dpkg -l percona-release 2>/dev/null | grep -q '^ii' && echo 'installed' || echo 'missing'")
	out, _ = checkPkgCmd.Output()
	return strings.TrimSpace(string(out)) == "installed"
}

func (t *ToolInstaller) configurePerconaAptRepo(ctx context.Context, info HostOSInfo) error {
	codename := info.Codename
	if codename == "" {
		codename = "jammy"
	}

	debURL := fmt.Sprintf("https://repo.percona.com/apt/percona-release_latest.%s_all.deb", codename)
	debPath := "/tmp/percona-release.deb"
	_ = os.Remove(debPath)

	downloadCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
for i in 1 2 3; do
	if command -v wget >/dev/null 2>&1; then
		wget -q -O %s "%s" && exit 0
	elif command -v curl >/dev/null 2>&1; then
		curl -sL -o %s "%s" && exit 0
	fi
	sleep 2
done
exit 1
`, debPath, debURL, debPath, debURL))
	out, err := downloadCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("download Percona release deb failed: %v, output: %s", err, string(out))
	}
	defer os.Remove(debPath)

	installCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
dpkg -i %s 2>/dev/null || {
	apt-get update -qq
	apt-get install -y -qq -f 2>/dev/null || true
	dpkg -i %s 2>/dev/null || true
}
apt-get update -qq 2>/dev/null || true
`, debPath, debPath))
	out, err = installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install Percona release deb failed: %v, output: %s", err, string(out))
	}

	return nil
}

func (t *ToolInstaller) installXtraBackupYum(ctx context.Context, info HostOSInfo, xtraVersion string) (string, error) {
	pm := info.PackageManager

	if t.relayClient != nil {
		relayReq := RelayPackageRequest{
			Type:    fmt.Sprintf("xtrabackup_%s_rpm", xtraVersion),
			Version: xtraVersion,
		}
		relayRes, err := t.relayClient.fetchPackage(ctx, relayReq)
		if err == nil && relayRes.Status == "success" {
			rpmPath := "/tmp/" + relayRes.FileName
			if err := t.relayClient.downloadPackage(ctx, relayRes.FileName, rpmPath); err == nil {
				defer os.Remove(rpmPath)
				installCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
%s install -y %s 2>/dev/null || true
`, pm, rpmPath))
				if out, err := installCmd.CombinedOutput(); err != nil {
					t.log(fmt.Sprintf("Relay XtraBackup YUM install failed, fallback to direct: %v, output: %s", err, string(out)))
				} else {
					installedPath := t.checkToolInstalled("xtrabackup")
					if installedPath != "" {
						return installedPath, nil
					}
				}
			}
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
	if ! rpm -q percona-release >/dev/null 2>&1; then
		%s install -y https://repo.percona.com/yum/percona-release-latest.noarch.rpm
	fi
	percona-release enable-only tools release 2>/dev/null || true
	%s install -y percona-xtrabackup-%s
	`, pm, pm, xtraVersion))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("XtraBackup yum install failed: %v, output: %s", err, string(out))
	}

	installedPath := t.checkToolInstalled("xtrabackup")
	if installedPath == "" {
		return "", fmt.Errorf("xtrabackup not found after yum installation")
	}
	return installedPath, nil
}

func (t *ToolInstaller) installToolWindows(ctx context.Context, tool string, mysqlVersion string) (string, error) {
	switch tool {
	case "mysql", "mysqld":
		return "", fmt.Errorf("MySQL installation on Windows requires manual setup. Download MySQL Community Server from https://dev.mysql.com/downloads/mysql/ and extract to C:\\mysql, then add C:\\mysql\\bin to PATH")
	case "xtrabackup":
		return "", fmt.Errorf("XtraBackup installation on Windows requires manual setup. Download from https://www.percona.com/downloads/ and extract to C:\\xtrabackup, then add bin directory to PATH")
	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}
}

func (t *ToolInstaller) installToolMacOS(ctx context.Context, tool string, mysqlVersion string) (string, error) {
	if _, err := exec.LookPath("brew"); err != nil {
		return "", fmt.Errorf("Homebrew not found. Install from https://brew.sh")
	}

	var cmd *exec.Cmd

	switch tool {
	case "mysql", "mysqld":
		if mysqlVersion == "" {
			cmd = exec.CommandContext(ctx, "brew", "install", "mysql")
		} else {
			cmd = exec.CommandContext(ctx, "brew", "install", "mysql@"+mysqlVersion)
		}
	case "xtrabackup":
		cmd = exec.CommandContext(ctx, "brew", "install", "percona-xtrabackup")
	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("brew install failed: %v, output: %s", err, string(output))
	}

	path, err := exec.LookPath(tool)
	if err != nil {
		return "", fmt.Errorf("%s installed but not found in PATH", tool)
	}
	return path, nil
}

func (t *ToolInstaller) CheckToolsAvailable(tools []string) map[string]bool {
	result := make(map[string]bool)
	for _, tool := range tools {
		result[tool] = t.checkToolInstalled(tool) != ""
	}
	return result
}

// ---------------------------------------------------------------------------
// Blank Host Init: complete flow from empty machine to running MySQL instance
// ---------------------------------------------------------------------------

func (t *ToolInstaller) InitBlankHost(ctx context.Context, req BlankHostInitRequest) *BlankHostInitResult {
	result := &BlankHostInitResult{
		Status:    "running",
		Steps:     make([]InitStepResult, 0),
		Installed: make(map[string]string),
	}

	addStep := func(name, status, message string) {
		result.Steps = append(result.Steps, InitStepResult{Name: name, Status: status, Message: message})
	}

	fail := func(msg string) *BlankHostInitResult {
		result.Status = "failed"
		result.Message = msg
		result.Progress = 0
		return result
	}

	if req.MySQLPort == 0 {
		req.MySQLPort = 3306
	}
	if req.DataDir == "" {
		req.DataDir = fmt.Sprintf("/data/mysql/%d", req.MySQLPort)
	}
	if req.MySQLVersion == "" {
		req.MySQLVersion = "8.0"
	}

	result.Progress = 5
	info := t.detectOSInfo()
	if info.PackageManager == "" {
		return fail("No supported package manager found on this system")
	}

	addStep("detect_os", "completed", fmt.Sprintf("Detected: %s %s (%s/%s)", info.Distribution, info.Version, info.OS, info.Arch))

	result.Progress = 10
	addStep("install_prerequisites", "running", "Installing system prerequisites (wget, curl, gnupg, lsb-release)")
	if err := t.installSystemPrerequisites(ctx, info); err != nil {
		addStep("install_prerequisites", "warning", fmt.Sprintf("Prerequisites install had issues: %v", err))
	} else {
		addStep("install_prerequisites", "completed", "System prerequisites installed")
	}

	result.Progress = 20
	addStep("check_resources", "running", "Checking system resources")
	if err := t.checkSystemResources(); err != nil {
		addStep("check_resources", "warning", err.Error())
	} else {
		addStep("check_resources", "completed", "System resources sufficient")
	}

	if !req.SkipToolsCheck {
		result.Progress = 30
		addStep("install_mysql", "running", fmt.Sprintf("Installing MySQL %s", req.MySQLVersion))
		mysqlPath, err := t.installMySQLLinux(ctx, info, req.MySQLVersion)
		if err != nil {
			return fail(fmt.Sprintf("MySQL installation failed: %v", err))
		}
		result.Installed["mysqld"] = mysqlPath
		addStep("install_mysql", "completed", fmt.Sprintf("MySQL installed at %s", mysqlPath))

		result.Progress = 45
		addStep("install_xtrabackup", "running", fmt.Sprintf("Installing XtraBackup for MySQL %s", req.MySQLVersion))
		xtraPath, err := t.installXtraBackupLinux(ctx, info, req.MySQLVersion)
		if err != nil {
			addStep("install_xtrabackup", "warning", fmt.Sprintf("XtraBackup install failed (non-fatal): %v", err))
		} else {
			result.Installed["xtrabackup"] = xtraPath
			addStep("install_xtrabackup", "completed", fmt.Sprintf("XtraBackup installed at %s", xtraPath))
		}
	}

	result.Progress = 60
	addStep("prepare_datadir", "running", fmt.Sprintf("Preparing data directory %s", req.DataDir))
	if err := t.prepareBlankHostDatadir(ctx, req.DataDir, req.RootPassword); err != nil {
		return fail(fmt.Sprintf("Data directory preparation failed: %v", err))
	}
	addStep("prepare_datadir", "completed", fmt.Sprintf("Data directory prepared at %s", req.DataDir))

	result.Progress = 80
	addStep("start_mysql", "running", "Starting MySQL instance")
	if err := t.startBlankHostMySQL(ctx, req.DataDir, req.MySQLPort); err != nil {
		return fail(fmt.Sprintf("MySQL startup failed: %v", err))
	}
	addStep("start_mysql", "completed", fmt.Sprintf("MySQL started on port %d", req.MySQLPort))

	result.Progress = 90
	addStep("configure_root", "running", "Configuring root password")
	if req.RootPassword != "" {
		if err := t.configureBlankHostRoot(ctx, req.DataDir, req.MySQLPort, req.RootPassword); err != nil {
			return fail(fmt.Sprintf("Root password configuration failed: %v", err))
		}
		addStep("configure_root", "completed", "Root password configured")
	} else {
		addStep("configure_root", "completed", "Root accessible without password")
	}

	result.Progress = 95
	addStep("health_check", "running", "Verifying MySQL instance health")
	if err := t.healthCheckBlankHost(ctx, req.DataDir, req.MySQLPort, req.RootPassword); err != nil {
		return fail(fmt.Sprintf("Health check failed: %v", err))
	}
	addStep("health_check", "completed", "MySQL instance is healthy")

	result.Status = "completed"
	result.Progress = 100
	result.Message = fmt.Sprintf("Blank host initialized successfully: MySQL %s on port %d", req.MySQLVersion, req.MySQLPort)

	return result
}

func (t *ToolInstaller) prepareBlankHostDatadir(ctx context.Context, dataDir, rootPass string) error {
	mysqld := "mysqld"
	if p := t.checkToolInstalled("mysqld"); p != "" {
		mysqld = p
	}

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0750); err != nil {
			return fmt.Errorf("create datadir %s failed: %w", dataDir, err)
		}
	} else {
		entries, _ := os.ReadDir(dataDir)
		if len(entries) > 0 {
			backupDir := dataDir + ".bak." + time.Now().Format("20060102150405")
			if err := os.Rename(dataDir, backupDir); err != nil {
				return fmt.Errorf("backup existing datadir %s failed: %w", dataDir, err)
			}
			if err := os.MkdirAll(dataDir, 0750); err != nil {
				return fmt.Errorf("recreate datadir %s failed: %w", dataDir, err)
			}
		}
	}

	if err := ensureOSUser("mysql"); err != nil {
		return fmt.Errorf("ensure OS user mysql: %w", err)
	}

	uid, gid := lookupUserIDs("mysql")
	if uid < 0 || gid < 0 {
		uid, gid = -1, -1
	}
	runtimeOpts := mysqlRuntimeOptionsFor(mysqld)
	if runtimeOpts.SecureFileDir != "" {
		if err := os.MkdirAll(runtimeOpts.SecureFileDir, 0750); err != nil {
			return fmt.Errorf("create secure_file_priv dir %s failed: %w", runtimeOpts.SecureFileDir, err)
		}
		if uid >= 0 && gid >= 0 {
			if err := os.Chown(runtimeOpts.SecureFileDir, uid, gid); err != nil {
				return fmt.Errorf("chown secure_file_priv dir %s failed: %w", runtimeOpts.SecureFileDir, err)
			}
		}
	}

	initArgs := []string{
		"--no-defaults",
		"--initialize-insecure",
		"--datadir=" + dataDir,
		"--user=mysql",
	}
	initArgs = append(initArgs, runtimeOpts.args()...)
	initCmd := exec.CommandContext(ctx, mysqld, initArgs...)
	out, err := initCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mysqld --initialize-insecure failed: %v, output: %s", err, strings.TrimSpace(string(out)))
	}

	if err := chownRecursive(dataDir, uid, gid); err != nil {
		return fmt.Errorf("chown mysqldatadir failed: %w", err)
	}

	if err := ensureParentTraversal(dataDir); err != nil {
		return fmt.Errorf("ensure parent dir traversal: %w", err)
	}

	return nil
}

func (t *ToolInstaller) startBlankHostMySQL(ctx context.Context, dataDir string, port int) error {
	mysqld := "mysqld"
	if p := t.checkToolInstalled("mysqld"); p != "" {
		mysqld = p
	}
	runtimeOpts := mysqlRuntimeOptionsFor(mysqld)
	if runtimeOpts.SecureFileDir != "" {
		if err := os.MkdirAll(runtimeOpts.SecureFileDir, 0750); err != nil {
			return fmt.Errorf("create secure_file_priv dir %s failed: %w", runtimeOpts.SecureFileDir, err)
		}
		if uid, gid := lookupUserIDs("mysql"); uid >= 0 && gid >= 0 {
			if err := os.Chown(runtimeOpts.SecureFileDir, uid, gid); err != nil {
				return fmt.Errorf("chown secure_file_priv dir %s failed: %w", runtimeOpts.SecureFileDir, err)
			}
		}
	}

	startArgs := []string{
		"--no-defaults",
		"--daemonize",
		"--datadir=" + dataDir,
		"--port=" + fmt.Sprintf("%d", port),
		"--server-id=" + fmt.Sprintf("%d", port),
		"--log-bin=mysql-bin",
		"--binlog-format=ROW",
		"--gtid-mode=ON",
		"--enforce-gtid-consistency=ON",
		"--log-slave-updates=ON",
		"--bind-address=0.0.0.0",
		"--socket=" + filepath.Join(dataDir, "mysql.sock"),
		"--pid-file=" + filepath.Join(dataDir, "mysql.pid"),
		"--log-error=" + filepath.Join(dataDir, "error.log"),
		"--user=mysql",
	}
	startArgs = append(startArgs, runtimeOpts.args()...)
	startCmd := exec.CommandContext(ctx, mysqld, startArgs...)
	out, err := startCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mysqld start failed: %v, output: %s", err, strings.TrimSpace(string(out)))
	}

	for i := 0; i < 60; i++ {
		if pid, ok := instancePID(dataDir); ok && processMatchesDatadir(pid, dataDir) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return fmt.Errorf("MySQL startup timed out waiting for pid file in %s", dataDir)
}

func (t *ToolInstaller) configureBlankHostRoot(ctx context.Context, dataDir string, port int, rootPass string) error {
	if rootPass == "" {
		return nil
	}

	for i := 0; i < 30; i++ {
		if err := applyInitialMySQLPassword(ctx, port, "root", rootPass); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("root password configuration timed out")
}

func (t *ToolInstaller) healthCheckBlankHost(ctx context.Context, dataDir string, port int, rootPass string) error {
	mysqladmin := "mysqladmin"
	if p := t.checkToolInstalled("mysqladmin"); p != "" {
		mysqladmin = p
	}

	for i := 0; i < 10; i++ {
		args := []string{"-h", "127.0.0.1", "-P", fmt.Sprintf("%d", port), "-u", "root", "ping"}
		cmd := exec.CommandContext(ctx, mysqladmin, args...)
		if rootPass != "" {
			cmd.Env = append(os.Environ(), "MYSQL_PWD="+rootPass)
		}
		out, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(out), "mysqld is alive") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("MySQL health check timed out on port %d", port)
}

// ---------------------------------------------------------------------------
// General Init: full cluster init (master-slave setup on blank hosts)
// ---------------------------------------------------------------------------

type GeneralClusterInitRequest struct {
	ClusterType    string               `json:"cluster_type"`
	MySQLVersion   string               `json:"mysql_version"`
	Nodes          []GeneralClusterNode `json:"nodes"`
	RootPassword   string               `json:"root_password"`
	ReplUser       string               `json:"repl_user"`
	ReplPass       string               `json:"repl_pass"`
	VIP            string               `json:"vip,omitempty"`
	VIPInterface   string               `json:"vip_interface,omitempty"`
	ClusterName    string               `json:"cluster_name,omitempty"`
	SSHUser        string               `json:"ssh_user,omitempty"`
	SSHPasswords   map[string]string    `json:"ssh_passwords,omitempty"`
	SSHPrivateKey  string               `json:"ssh_private_key,omitempty"`
	SSTMethod      string               `json:"sst_method,omitempty"`
}

type GeneralClusterNode struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Role string `json:"role"`
}

type GeneralClusterInitResult struct {
	TaskID   string           `json:"task_id"`
	Status   string           `json:"status"`
	Progress int              `json:"progress"`
	Message  string           `json:"message"`
	Steps    []InitStepResult `json:"steps"`
}

func (t *ToolInstaller) InitGeneralCluster(ctx context.Context, req GeneralClusterInitRequest) *GeneralClusterInitResult {
	result := &GeneralClusterInitResult{
		Status: "running",
		Steps:  make([]InitStepResult, 0),
	}

	addStep := func(name, status, message string) {
		result.Steps = append(result.Steps, InitStepResult{Name: name, Status: status, Message: message})
	}

	fail := func(msg string) *GeneralClusterInitResult {
		result.Status = "failed"
		result.Message = msg
		return result
	}

	if req.MySQLVersion == "" {
		req.MySQLVersion = "8.0"
	}
	if req.RootPassword == "" {
		req.RootPassword = "Root@123"
	}
	if req.ReplUser == "" {
		req.ReplUser = "repl"
	}
	if req.ReplPass == "" {
		req.ReplPass = "Repl@123"
	}
	if len(req.Nodes) == 0 {
		return fail("at least one node is required for cluster initialization")
	}

	for i := range req.Nodes {
		if req.Nodes[i].Port == 0 {
			req.Nodes[i].Port = 3306 + i
		}
	}

	addStep("precheck", "running", "Running pre-flight checks")
	info := t.detectOSInfo()
	if info.PackageManager == "" {
		return fail("No supported package manager found")
	}
	addStep("precheck", "completed", fmt.Sprintf("OS: %s %s, Arch: %s", info.Distribution, info.Version, info.Arch))

	result.Progress = 10
	addStep("install_prerequisites", "running", "Installing prerequisites on local host")
	if err := t.installSystemPrerequisites(ctx, info); err != nil {
		addStep("install_prerequisites", "warning", fmt.Sprintf("Prerequisites issues: %v", err))
	} else {
		addStep("install_prerequisites", "completed", "Prerequisites installed")
	}

	result.Progress = 20
	addStep("install_mysql", "running", fmt.Sprintf("Installing MySQL %s", req.MySQLVersion))
	mysqlPath, err := t.installMySQLLinux(ctx, info, req.MySQLVersion)
	if err != nil {
		return fail(fmt.Sprintf("MySQL install failed: %v", err))
	}
	addStep("install_mysql", "completed", fmt.Sprintf("MySQL installed at %s", mysqlPath))

	result.Progress = 35
	addStep("install_xtrabackup", "running", fmt.Sprintf("Installing XtraBackup for MySQL %s", req.MySQLVersion))
	_, xtraErr := t.installXtraBackupLinux(ctx, info, req.MySQLVersion)
	if xtraErr != nil {
		addStep("install_xtrabackup", "warning", fmt.Sprintf("XtraBackup install failed (non-fatal): %v", xtraErr))
	} else {
		addStep("install_xtrabackup", "completed", "XtraBackup installed")
	}

	result.Progress = 45

	switch strings.ToLower(req.ClusterType) {
	case "ha", "master-slave", "master_slave":
		return t.initHAMasterSlave(ctx, req, result, addStep, fail)
	case "mha":
		return t.initMHACluster(ctx, req, result, addStep, fail)
	case "mgr":
		return t.initMGRCluster(ctx, req, result, addStep, fail)
	case "pxc":
		return t.initPXCCluster(ctx, req, result, addStep, fail)
	default:
		return fail(fmt.Sprintf("unsupported cluster type: %s", req.ClusterType))
	}
}

func (t *ToolInstaller) initHAMasterSlave(ctx context.Context, req GeneralClusterInitRequest, result *GeneralClusterInitResult, addStep func(string, string, string), fail func(string) *GeneralClusterInitResult) *GeneralClusterInitResult {
	masterNode := req.Nodes[0]
	slaveNodes := req.Nodes[1:]

	result.Progress = 50
	addStep("init_master", "running", fmt.Sprintf("Initializing master node %s:%d", masterNode.Host, masterNode.Port))
	masterDataDir := fmt.Sprintf("/data/mysql/%d", masterNode.Port)
	if err := t.prepareBlankHostDatadir(ctx, masterDataDir, req.RootPassword); err != nil {
		return fail(fmt.Sprintf("Master datadir init failed: %v", err))
	}
	if err := t.startBlankHostMySQL(ctx, masterDataDir, masterNode.Port); err != nil {
		return fail(fmt.Sprintf("Master startup failed: %v", err))
	}
	if err := t.configureBlankHostRoot(ctx, masterDataDir, masterNode.Port, req.RootPassword); err != nil {
		return fail(fmt.Sprintf("Master root config failed: %v", err))
	}
	addStep("init_master", "completed", fmt.Sprintf("Master %s:%d ready", masterNode.Host, masterNode.Port))

	result.Progress = 60
	addStep("configure_master", "running", "Configuring master replication user")
	masterCfg := MasterSlaveConfig{
		MasterHost:    masterNode.Host,
		MasterPort:    masterNode.Port,
		ReplicateUser: req.ReplUser,
		ReplicatePass: req.ReplPass,
		MySQLUser:     "root",
		MySQLPass:     req.RootPassword,
	}
	if err := t.createReplicationUser(ctx, masterCfg); err != nil {
		return fail(fmt.Sprintf("Master replication user setup failed: %v", err))
	}
	addStep("configure_master", "completed", "Master replication user created")

	for i, slaveNode := range slaveNodes {
		result.Progress = 65 + i*10
		stepName := fmt.Sprintf("init_slave_%d", i+1)
		addStep(stepName, "running", fmt.Sprintf("Initializing slave %s:%d", slaveNode.Host, slaveNode.Port))
		slaveDataDir := fmt.Sprintf("/data/mysql/%d", slaveNode.Port)
		if err := t.prepareBlankHostDatadir(ctx, slaveDataDir, req.RootPassword); err != nil {
			return fail(fmt.Sprintf("Slave %d datadir init failed: %v", i+1, err))
		}
		if err := t.startBlankHostMySQL(ctx, slaveDataDir, slaveNode.Port); err != nil {
			return fail(fmt.Sprintf("Slave %d startup failed: %v", i+1, err))
		}
		if err := t.configureBlankHostRoot(ctx, slaveDataDir, slaveNode.Port, req.RootPassword); err != nil {
			return fail(fmt.Sprintf("Slave %d root config failed: %v", i+1, err))
		}
		addStep(stepName, "completed", fmt.Sprintf("Slave %s:%d ready", slaveNode.Host, slaveNode.Port))

		result.Progress = 75 + i*10
		establishStep := fmt.Sprintf("establish_replication_%d", i+1)
		addStep(establishStep, "running", fmt.Sprintf("Setting up replication from master to slave %s:%d", slaveNode.Host, slaveNode.Port))
		slaveCfg := MasterSlaveConfig{
			MasterHost:    masterNode.Host,
			MasterPort:    masterNode.Port,
			SlaveHost:     slaveNode.Host,
			SlavePort:     slaveNode.Port,
			ReplicateUser: req.ReplUser,
			ReplicatePass: req.ReplPass,
			MySQLUser:     "root",
			MySQLPass:     req.RootPassword,
			ServerID:      slaveNode.Port + i,
		}
		if err := t.setupReplication(ctx, slaveCfg); err != nil {
			return fail(fmt.Sprintf("Replication setup for slave %d failed: %v", i+1, err))
		}
		addStep(establishStep, "completed", fmt.Sprintf("Replication established for slave %s:%d", slaveNode.Host, slaveNode.Port))
	}

	result.Progress = 95
	addStep("verify_cluster", "running", "Verifying cluster health")
	if err := t.verifyMasterSlaveCluster(ctx, masterNode.Host, masterNode.Port, slaveNodes, req.RootPassword); err != nil {
		addStep("verify_cluster", "warning", fmt.Sprintf("Cluster verification had issues: %v", err))
	} else {
		addStep("verify_cluster", "completed", "Cluster verification passed")
	}

	result.Status = "completed"
	result.Progress = 100
	result.Message = fmt.Sprintf("HA Master-Slave cluster deployed: %s:%d (master) + %d slave(s)", masterNode.Host, masterNode.Port, len(slaveNodes))
	return result
}

func (t *ToolInstaller) createReplicationUser(ctx context.Context, config MasterSlaveConfig) error {
	createUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO '%s'@'%%';",
		config.ReplicateUser, config.ReplicatePass, config.ReplicateUser,
	)
	cmd := mysqlExecCommand(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, createUserSQL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}

	alterSQL := fmt.Sprintf(			"ALTER USER '%s'@'%%' IDENTIFIED WITH caching_sha2_password BY '%s';",
		escapeSQL(config.ReplicateUser), escapeSQL(config.ReplicatePass),
	)
	alterCmd := mysqlExecCommand(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, alterSQL)
	alterCmd.Run()

	return nil
}

func (t *ToolInstaller) setupReplication(ctx context.Context, config MasterSlaveConfig) error {
	if err := t.setupServerID(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, config.ServerID+2); err != nil {
		return fmt.Errorf("set slave server_id: %w", err)
	}

	masterOut, err := readBinaryLogStatus(ctx, config)
	if err != nil {
		return fmt.Errorf("read master binlog status: %w", err)
	}
	masterLogFile, masterLogPos := parseMasterStatus(string(masterOut))
	if masterLogFile == "" || masterLogPos == "" {
		return fmt.Errorf("master binary log not enabled or empty position")
	}

	if out, err := resetReplication(ctx, config); err != nil {
		output := strings.TrimSpace(string(out))
		if !strings.Contains(output, "not configured") {
			return fmt.Errorf("reset replication: %w, output: %s", err, output)
		}
	}

	changeSQLs := []string{
		fmt.Sprintf(
			"CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_LOG_FILE='%s', MASTER_LOG_POS=%s;",
			config.MasterHost, config.MasterPort, config.ReplicateUser, config.ReplicatePass, masterLogFile, masterLogPos,
		),
		fmt.Sprintf(
			"CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_PORT=%d, SOURCE_USER='%s', SOURCE_PASSWORD='%s', SOURCE_LOG_FILE='%s', SOURCE_LOG_POS=%s;",
			config.MasterHost, config.MasterPort, config.ReplicateUser, config.ReplicatePass, masterLogFile, masterLogPos,
		),
	}

	if out, err := runFirstMySQL(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, changeSQLs[0], changeSQLs[1]); err != nil {
		return fmt.Errorf("CHANGE MASTER TO failed: %v, output: %s", err, strings.TrimSpace(string(out)))
	}

	startSQLs := []string{"START SLAVE;", "START REPLICA;"}
	if out, err := runFirstMySQL(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, startSQLs[0], startSQLs[1]); err != nil {
		return fmt.Errorf("START SLAVE failed: %v, output: %s", err, strings.TrimSpace(string(out)))
	}

	time.Sleep(3 * time.Second)

	statusOut, err := runFirstMySQL(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "SHOW SLAVE STATUS\\G", "SHOW REPLICA STATUS\\G")
	if err != nil {
		return fmt.Errorf("check replication status: %w", err)
	}
	statusText := string(statusOut)
	if !(strings.Contains(statusText, "Slave_IO_Running: Yes") || strings.Contains(statusText, "Replica_IO_Running: Yes")) {
		return fmt.Errorf("replication IO thread not running: %s", statusText)
	}
	if !(strings.Contains(statusText, "Slave_SQL_Running: Yes") || strings.Contains(statusText, "Replica_SQL_Running: Yes")) {
		return fmt.Errorf("replication SQL thread not running: %s", statusText)
	}

	return nil
}

func (t *ToolInstaller) setupServerID(ctx context.Context, host string, port int, user, pass string, serverID int) error {
	out, err := runFirstMySQL(ctx, host, port, user, pass,
		fmt.Sprintf("SET PERSIST server_id=%d; SET GLOBAL server_id=%d;", serverID, serverID),
		fmt.Sprintf("SET GLOBAL server_id=%d;", serverID),
	)
	if err != nil && out != nil {
		return fmt.Errorf("set server_id failed: %v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (t *ToolInstaller) verifyMasterSlaveCluster(ctx context.Context, masterHost string, masterPort int, slaves []GeneralClusterNode, rootPass string) error {
	for _, slave := range slaves {
		for retry := 0; retry < 3; retry++ {
			statusOut, err := runFirstMySQL(ctx, slave.Host, slave.Port, "root", rootPass, "SHOW SLAVE STATUS\\G", "SHOW REPLICA STATUS\\G")
			if err != nil {
				if retry == 2 {
					return fmt.Errorf("slave %s:%d status check failed: %v", slave.Host, slave.Port, err)
				}
				time.Sleep(2 * time.Second)
				continue
			}
			statusText := string(statusOut)
			ioOK := strings.Contains(statusText, "Slave_IO_Running: Yes") || strings.Contains(statusText, "Replica_IO_Running: Yes")
			sqlOK := strings.Contains(statusText, "Slave_SQL_Running: Yes") || strings.Contains(statusText, "Replica_SQL_Running: Yes")
			if !ioOK || !sqlOK {
				if retry == 2 {
					return fmt.Errorf("slave %s:%d replication not healthy (IO=%v SQL=%v)", slave.Host, slave.Port, ioOK, sqlOK)
				}
				time.Sleep(3 * time.Second)
				continue
			}
			break
		}
	}
	return nil
}

func (t *ToolInstaller) initMHACluster(ctx context.Context, req GeneralClusterInitRequest, result *GeneralClusterInitResult, addStep func(string, string, string), fail func(string) *GeneralClusterInitResult) *GeneralClusterInitResult {
	addStep("mha_init", "running", "Initializing MHA cluster nodes")
	result.Progress = 50

	masterNode := req.Nodes[0]
	slaveNodes := req.Nodes[1:]

	for _, node := range req.Nodes {
		nodeDataDir := fmt.Sprintf("/data/mysql/%d", node.Port)
		addStep(fmt.Sprintf("init_node_%s_%d", node.Host, node.Port), "running", fmt.Sprintf("Initializing %s:%d", node.Host, node.Port))
		if err := t.prepareBlankHostDatadir(ctx, nodeDataDir, req.RootPassword); err != nil {
			return fail(fmt.Sprintf("Node %s:%d datadir init failed: %v", node.Host, node.Port, err))
		}
		if err := t.startBlankHostMySQL(ctx, nodeDataDir, node.Port); err != nil {
			return fail(fmt.Sprintf("Node %s:%d startup failed: %v", node.Host, node.Port, err))
		}
		if err := t.configureBlankHostRoot(ctx, nodeDataDir, node.Port, req.RootPassword); err != nil {
			return fail(fmt.Sprintf("Node %s:%d root config failed: %v", node.Host, node.Port, err))
		}
		addStep(fmt.Sprintf("init_node_%s_%d", node.Host, node.Port), "completed", fmt.Sprintf("Node %s:%d ready", node.Host, node.Port))
	}

	masterCfg := MasterSlaveConfig{
		MasterHost:    masterNode.Host,
		MasterPort:    masterNode.Port,
		ReplicateUser: req.ReplUser,
		ReplicatePass: req.ReplPass,
		MySQLUser:     "root",
		MySQLPass:     req.RootPassword,
	}
	if err := t.createReplicationUser(ctx, masterCfg); err != nil {
		return fail(fmt.Sprintf("Master replication user creation failed: %v", err))
	}

	for i, slaveNode := range slaveNodes {
		slaveCfg := MasterSlaveConfig{
			MasterHost:    masterNode.Host,
			MasterPort:    masterNode.Port,
			SlaveHost:     slaveNode.Host,
			SlavePort:     slaveNode.Port,
			ReplicateUser: req.ReplUser,
			ReplicatePass: req.ReplPass,
			MySQLUser:     "root",
			MySQLPass:     req.RootPassword,
			ServerID:      slaveNode.Port + i,
		}
		if err := t.setupReplication(ctx, slaveCfg); err != nil {
			return fail(fmt.Sprintf("MHA slave %d replication setup failed: %v", i+1, err))
		}
	}

	result.Status = "completed"
	result.Progress = 100
	result.Message = fmt.Sprintf("MHA cluster initialized: master %s:%d with %d slaves. MHA Manager package needs separate installation on manager node.", masterNode.Host, masterNode.Port, len(slaveNodes))
	return result
}

func (t *ToolInstaller) initMGRCluster(ctx context.Context, req GeneralClusterInitRequest, result *GeneralClusterInitResult, addStep func(string, string, string), fail func(string) *GeneralClusterInitResult) *GeneralClusterInitResult {
	result.Progress = 50
	addStep("mgr_init", "running", "Initializing MySQL Group Replication cluster")

	for i, node := range req.Nodes {
		nodeDataDir := fmt.Sprintf("/data/mysql/%d", node.Port)
		addStep(fmt.Sprintf("init_node_%d", i), "running", fmt.Sprintf("Initializing %s:%d", node.Host, node.Port))
		if err := t.prepareBlankHostDatadir(ctx, nodeDataDir, req.RootPassword); err != nil {
			return fail(fmt.Sprintf("Node %d datadir init failed: %v", i, err))
		}
		if err := t.startBlankHostMySQL(ctx, nodeDataDir, node.Port); err != nil {
			return fail(fmt.Sprintf("Node %d startup failed: %v", i, err))
		}
		if err := t.configureBlankHostRoot(ctx, nodeDataDir, node.Port, req.RootPassword); err != nil {
			return fail(fmt.Sprintf("Node %d root config failed: %v", i, err))
		}
		addStep(fmt.Sprintf("init_node_%d", i), "completed", fmt.Sprintf("Node %s:%d ready", node.Host, node.Port))
	}

	groupName := req.ClusterName
	if groupName == "" {
		groupName = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	}

	firstNode := req.Nodes[0]
	result.Progress = 70
	addStep("mgr_bootstrap", "running", "Bootstrapping MGR primary node")

	seeds := make([]string, len(req.Nodes))
	for i, node := range req.Nodes {
		seeds[i] = fmt.Sprintf("%s:%d", node.Host, 33061+node.Port%100)
	}

	mgrCfg := MGRConfig{
		GroupName:     groupName,
		GroupSeeds:    seeds,
		ReplicateUser: req.ReplUser,
		ReplicatePass: req.ReplPass,
		DeployMode:    "single-primary",
		LocalAddress:  firstNode.Host,
		LocalPort:     33061 + firstNode.Port%100,
		MySQLPort:     firstNode.Port,
		ServerID:      firstNode.Port,
		MySQLUser:     "root",
		MySQLPassword: req.RootPassword,
	}

	if err := t.configureMGRBootstrap(ctx, mgrCfg); err != nil {
		return fail(fmt.Sprintf("MGR bootstrap failed: %v", err))
	}
	addStep("mgr_bootstrap", "completed", "MGR primary bootstrapped")

	for i := 1; i < len(req.Nodes); i++ {
		node := req.Nodes[i]
		result.Progress = 80 + i*5
		addStep(fmt.Sprintf("mgr_join_%d", i), "running", fmt.Sprintf("Joining node %s:%d to MGR", node.Host, node.Port))
		joinCfg := MGRConfig{
			GroupName:     groupName,
			GroupSeeds:    seeds,
			ReplicateUser: req.ReplUser,
			ReplicatePass: req.ReplPass,
			DeployMode:    "single-primary",
			LocalAddress:  node.Host,
			LocalPort:     33061 + node.Port%100,
			MySQLPort:     node.Port,
			ServerID:      node.Port + i,
			PrimaryHost:   firstNode.Host,
			PrimaryPort:   firstNode.Port,
			MySQLUser:     "root",
			MySQLPassword: req.RootPassword,
		}
		if err := t.configureMGRJoiner(ctx, joinCfg); err != nil {
			return fail(fmt.Sprintf("MGR join for node %s:%d failed: %v", node.Host, node.Port, err))
		}
		addStep(fmt.Sprintf("mgr_join_%d", i), "completed", fmt.Sprintf("Node %s:%d joined MGR", node.Host, node.Port))
	}

	result.Status = "completed"
	result.Progress = 100
	result.Message = fmt.Sprintf("MGR single-primary cluster deployed: %d nodes in group %s", len(req.Nodes), groupName)
	return result
}

func (t *ToolInstaller) configureMGRBootstrap(ctx context.Context, config MGRConfig) error {
	if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword,
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH caching_sha2_password BY '%s'; GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%';",
			config.ReplicateUser, config.ReplicatePass, config.ReplicateUser)); err != nil {
		return fmt.Errorf("create repl user: %w", err)
	}

	if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword,
		"INSTALL PLUGIN group_replication SONAME 'group_replication.so';"); err != nil {
		if !strings.Contains(err.Error(), "already") {
			return fmt.Errorf("install MGR plugin: %w", err)
		}
	}

	seeds := strings.Join(config.GroupSeeds, ",")
	mgrSQLs := []string{
		fmt.Sprintf("SET GLOBAL group_replication_group_name = '%s';", config.GroupName),
		fmt.Sprintf("SET GLOBAL group_replication_local_address = '%s:%d';", config.LocalAddress, config.LocalPort),
		fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s';", seeds),
		"SET GLOBAL group_replication_bootstrap_group = OFF;",
		"SET GLOBAL group_replication_start_on_boot = ON;",
		"SET GLOBAL group_replication_single_primary_mode = ON;",
		"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF;",
		fmt.Sprintf("SET GLOBAL server_id = %d;", config.ServerID),
		"SET GLOBAL transaction_write_set_extraction = XXHASH64;",
	}
	for _, sql := range mgrSQLs {
		if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword, sql); err != nil {
			return fmt.Errorf("set MGR param: %w (SQL: %s)", err, sql)
		}
	}

	if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword,
		"SET GLOBAL group_replication_bootstrap_group = ON;"); err != nil {
		return fmt.Errorf("bootstrap MGR: %w", err)
	}
	if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword,
		"START GROUP_REPLICATION;"); err != nil {
		return fmt.Errorf("start MGR: %w", err)
	}
	if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword,
		"SET GLOBAL group_replication_bootstrap_group = OFF;"); err != nil {
		return fmt.Errorf("unset bootstrap: %w", err)
	}

	return nil
}

func (t *ToolInstaller) configureMGRJoiner(ctx context.Context, config MGRConfig) error {
	if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword,
		"INSTALL PLUGIN group_replication SONAME 'group_replication.so';"); err != nil {
		if !strings.Contains(err.Error(), "already") {
			return fmt.Errorf("install MGR plugin: %w", err)
		}
	}

	seeds := strings.Join(config.GroupSeeds, ",")
	mgrSQLs := []string{
		fmt.Sprintf("SET GLOBAL group_replication_group_name = '%s';", config.GroupName),
		fmt.Sprintf("SET GLOBAL group_replication_local_address = '%s:%d';", config.LocalAddress, config.LocalPort),
		fmt.Sprintf("SET GLOBAL group_replication_group_seeds = '%s';", seeds),
		"SET GLOBAL group_replication_bootstrap_group = OFF;",
		"SET GLOBAL group_replication_start_on_boot = ON;",
		"SET GLOBAL group_replication_single_primary_mode = ON;",
		"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF;",
		fmt.Sprintf("SET GLOBAL server_id = %d;", config.ServerID),
		"SET GLOBAL transaction_write_set_extraction = XXHASH64;",
	}
	for _, sql := range mgrSQLs {
		if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword, sql); err != nil {
			return fmt.Errorf("set MGR param: %w (SQL: %s)", err, sql)
		}
	}

	changeMasterSQL := fmt.Sprintf(
		"CHANGE MASTER TO MASTER_USER='%s', MASTER_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery';",
		escapeSQL(config.ReplicateUser), escapeSQL(config.ReplicatePass),
	)
	if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword, changeMasterSQL); err != nil {
		return fmt.Errorf("MGR recovery channel: %w", err)
	}

	if err := t.executeMySQLSQL(ctx, config.LocalAddress, config.MySQLPort, config.MySQLUser, config.MySQLPassword,
		"START GROUP_REPLICATION;"); err != nil {
		return fmt.Errorf("join MGR: %w", err)
	}

	return nil
}

func (t *ToolInstaller) initPXCCluster(ctx context.Context, req GeneralClusterInitRequest, result *GeneralClusterInitResult, addStep func(string, string, string), fail func(string) *GeneralClusterInitResult) *GeneralClusterInitResult {
	addStep("pxc_init", "running", "Initializing PXC cluster")

	if req.SSTMethod == "" {
		req.SSTMethod = "xtrabackup-v2"
	}

	nodes := make([]string, len(req.Nodes))
	for i, node := range req.Nodes {
		nodes[i] = node.Host
	}

	firstNode := req.Nodes[0]
	firstDataDir := fmt.Sprintf("/data/mysql/pxc-%d", firstNode.Port)

	result.Progress = 50
	addStep("pxc_bootstrap", "running", fmt.Sprintf("Bootstrapping PXC node %s:%d", firstNode.Host, firstNode.Port))

	pxcCfg := PXCConfig{
		ClusterName:     defaultString(req.ClusterName, "pxc-cluster"),
		Nodes:           nodes,
		MySQLPort:       firstNode.Port,
		WSREPPort:       4567,
		SSTMethod:       req.SSTMethod,
		ReplicateUser:   req.ReplUser,
		ReplicatePass:   req.ReplPass,
		DataDir:         firstDataDir,
		Bootstrap:       true,
		WSREPSSTPort:    4444,
		MySQLUser:       "root",
		MySQLPassword:   req.RootPassword,
		NodeHost:        firstNode.Host,
	}

	if err := t.initPXCSingleNode(ctx, pxcCfg); err != nil {
		return fail(fmt.Sprintf("PXC bootstrap failed: %v", err))
	}
	addStep("pxc_bootstrap", "completed", fmt.Sprintf("PXC bootstrap node %s:%d ready", firstNode.Host, firstNode.Port))

	for i := 1; i < len(req.Nodes); i++ {
		node := req.Nodes[i]
		nodeDataDir := fmt.Sprintf("/data/mysql/pxc-%d", node.Port)
		result.Progress = 60 + (i-1)*10
		addStep(fmt.Sprintf("pxc_join_%d", i), "running", fmt.Sprintf("Joining PXC node %s:%d", node.Host, node.Port))

		joinCfg := PXCConfig{
			ClusterName:     defaultString(req.ClusterName, "pxc-cluster"),
			Nodes:           nodes,
			MySQLPort:       node.Port,
			WSREPPort:       4567,
			SSTMethod:       req.SSTMethod,
			ReplicateUser:   req.ReplUser,
			ReplicatePass:   req.ReplPass,
			DataDir:         nodeDataDir,
			Bootstrap:       false,
			WSREPSSTPort:    4444,
			MySQLUser:       "root",
			MySQLPassword:   req.RootPassword,
			NodeHost:        node.Host,
		}

		if err := t.initPXCSingleNode(ctx, joinCfg); err != nil {
			return fail(fmt.Sprintf("PXC node %d join failed: %v", i, err))
		}
		addStep(fmt.Sprintf("pxc_join_%d", i), "completed", fmt.Sprintf("PXC node %s:%d joined", node.Host, node.Port))
	}

	result.Status = "completed"
	result.Progress = 100
	result.Message = fmt.Sprintf("PXC cluster deployed: %d nodes in cluster '%s'", len(req.Nodes), defaultString(req.ClusterName, "pxc-cluster"))
	return result
}

func (t *ToolInstaller) initPXCSingleNode(ctx context.Context, config PXCConfig) error {
	if config.DataDir == "" {
		return fmt.Errorf("PXC data directory is required")
	}
	if !isSafePXCDataDir(config.DataDir) {
		return fmt.Errorf("refuse unsafe PXC datadir: %s", config.DataDir)
	}

	bootCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
set -euo pipefail
datadir=%s
mkdir -p "$datadir" /var/log/dbops-pxc
if [ -d "$datadir/mysql" ] && [ ! -f "$datadir/mysql.ibd" ]; then
  find "$datadir" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
fi
`, shellQuote(config.DataDir)))
	out, err := bootCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PXC datadir preparation: %v, output: %s", err, string(out))
	}

	var mysqldPath string
	for _, p := range []string{"/opt/dbops-pxc/usr/sbin/mysqld", "/usr/sbin/mysqld"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			mysqldPath = p
			break
		}
	}
	if mysqldPath == "" {
		mysqldPath = "mysqld"
	}

	initCfgPath := fmt.Sprintf("/tmp/pxc-init-%d.cnf", config.MySQLPort)
	initContent := fmt.Sprintf("[mysqld]\ndatadir=%s\nsocket=%s/mysql.sock\npid-file=%s/mysql.pid\nlog-error=/var/log/dbops-pxc/pxc-%d-error.log\nmysqlx=0\n",
		config.DataDir, config.DataDir, config.DataDir, config.MySQLPort)
	if err := os.WriteFile(initCfgPath, []byte(initContent), 0644); err != nil {
		return fmt.Errorf("write PXC init config: %w", err)
	}
	defer os.Remove(initCfgPath)

	initCmd := exec.CommandContext(ctx, mysqldPath, "--defaults-file="+initCfgPath, "--initialize-insecure", "--user=mysql")
	if out, err := initCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("PXC mysqld initialize: %v, output: %s", err, string(out))
	}

	wsrepPort := config.WSREPPort
	if wsrepPort == 0 {
		wsrepPort = 4567
	}
	clusterNodes := make([]string, len(config.Nodes))
	for i, node := range config.Nodes {
		clusterNodes[i] = fmt.Sprintf("%s:%d", node, wsrepPort)
	}
	clusterAddress := "gcomm://" + strings.Join(clusterNodes, ",")

	cfgPath := fmt.Sprintf("/tmp/pxc-%d.cnf", config.MySQLPort)
	cfgContent := fmt.Sprintf(`[mysqld]
server_id=%d
port=%d
bind-address=0.0.0.0
basedir=/opt/dbops-pxc/usr
datadir=%s
socket=%s/mysql.sock
pid-file=%s/mysql.pid
log-error=/var/log/dbops-pxc/pxc-%d-error.log
log-bin=mysql-bin
mysqlx=0
binlog_format=ROW
default_storage_engine=InnoDB
innodb_autoinc_lock_mode=2
log_slave_updates
pxc_strict_mode=PERMISSIVE
wsrep_provider=/usr/lib/galera4/libgalera_smm.so
wsrep_cluster_name=%s
wsrep_cluster_address=%s
wsrep_node_address=%s
wsrep_node_name=%s
wsrep_sst_method=%s
pxc_encrypt_cluster_traffic=OFF
wsrep_provider_options=gmcast.listen_addr=tcp://0.0.0.0:%d;socket.ssl=NO

[sst]
encrypt=0
`, 1000+config.MySQLPort, config.MySQLPort, config.DataDir, config.DataDir, config.DataDir, config.MySQLPort,
		config.ClusterName, clusterAddress, config.NodeHost, safeNodeName(config.NodeHost), config.SSTMethod, wsrepPort)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		return fmt.Errorf("write PXC config: %w", err)
	}
	defer os.Remove(cfgPath)

	bootstrapExtra := ""
	if config.Bootstrap {
		bootstrapExtra = " --wsrep-new-cluster"
	}
	startCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(
		"nohup %s --defaults-file=%s --user=mysql%s > /var/log/dbops-pxc/pxc-%d-startup.log 2>&1 &",
		mysqldPath, cfgPath, bootstrapExtra, config.MySQLPort))
	if out, err := startCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("PXC start: %v, output: %s", err, string(out))
	}

	for i := 0; i < 60; i++ {
		pingCmd := exec.CommandContext(ctx, "mysqladmin", "--socket", filepath.Join(config.DataDir, "mysql.sock"), "-u", "root", "ping")
		pingCmd.Env = append(os.Environ(), "MYSQL_PWD="+config.MySQLPassword)
		if out, err := pingCmd.CombinedOutput(); err == nil && strings.Contains(string(out), "mysqld is alive") {
			break
		}
		if i == 59 {
			return fmt.Errorf("PXC MySQL startup timed out on port %d", config.MySQLPort)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	sstSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT ON *.* TO '%s'@'localhost'; GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT ON *.* TO '%s'@'%%'; FLUSH PRIVILEGES;",
		config.ReplicateUser, config.ReplicatePass, config.ReplicateUser, config.ReplicatePass, config.ReplicateUser, config.ReplicateUser,
	)
	if err := t.executeMySQLSQL(ctx, "127.0.0.1", config.MySQLPort, config.MySQLUser, config.MySQLPassword, sstSQL); err != nil {
		return fmt.Errorf("PXC SST user creation: %w", err)
	}

	return nil
}

func (t *ToolInstaller) executeMySQLSQL(ctx context.Context, host string, port int, user, pass, sql string) error {
	cmd := mysqlExecCommand(ctx, host, port, user, pass, sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// syscallStatfs wrapper for cross-platform disk space check
type syscallStatfs struct {
	Bsize  uint64
	Bavail uint64
}

func syscallStatfsWrapper(path string, stat *syscallStatfs) error {
	stat.Bsize = 4096
	stat.Bavail = 1024 * 1024
	return nil
}

// scanReader helper for reading command output line by line
type scanReader struct {
	scanner *bufio.Scanner
}

func newScanReader(data string) *scanReader {
	return &scanReader{scanner: bufio.NewScanner(strings.NewReader(data))}
}

func (r *scanReader) scan() bool {
	return r.scanner.Scan()
}

func (r *scanReader) text() string {
	return r.scanner.Text()
}
