package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type AgentBinaryBuildReport struct {
	AgentDir    string
	LinuxPath   string
	WindowsPath string
}

func EnsureAgentBinaries() (*AgentBinaryBuildReport, error) {
	agentDir, err := findAgentProjectDir()
	if err != nil {
		return nil, err
	}
	binDir := filepath.Join(agentDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return nil, fmt.Errorf("create agent bin dir: %w", err)
	}

	linuxPath := filepath.Join(binDir, "agent-linux-amd64")
	windowsPath := filepath.Join(binDir, "agent-windows-amd64.exe")

	if hasAgentSource(agentDir) {
		goExe, err := findGoExecutable()
		if err != nil {
			return nil, err
		}
		if err := buildAgentBinary(goExe, agentDir, linuxPath, "linux", "amd64"); err != nil {
			return nil, err
		}
		if err := buildAgentBinary(goExe, agentDir, windowsPath, "windows", "amd64"); err != nil {
			return nil, err
		}
		if err := copyFile(linuxPath, filepath.Join(binDir, "agent")); err != nil {
			return nil, err
		}
		if err := copyFile(linuxPath, filepath.Join(binDir, "mysql-ops-agent-linux")); err != nil {
			return nil, err
		}
		_ = copyFile(windowsPath, filepath.Join(binDir, "agent.exe"))
	}

	if ok, err := isLinuxELFFile(linuxPath); !ok {
		if err != nil {
			return nil, fmt.Errorf("linux agent binary is invalid: %w", err)
		}
		return nil, fmt.Errorf("linux agent binary is not a valid ELF executable: %s", linuxPath)
	}
	if ok, err := isWindowsPEFile(windowsPath); !ok {
		if err != nil {
			return nil, fmt.Errorf("windows agent binary is invalid: %w", err)
		}
		return nil, fmt.Errorf("windows agent binary is not a valid PE executable: %s", windowsPath)
	}

	return &AgentBinaryBuildReport{
		AgentDir:    agentDir,
		LinuxPath:   linuxPath,
		WindowsPath: windowsPath,
	}, nil
}

func findAgentProjectDir() (string, error) {
	candidates := []string{
		filepath.Join("agent"),
		filepath.Join("..", "agent"),
		filepath.Join("..", "..", "agent"),
	}
	for _, candidate := range candidates {
		if hasAgentProject(candidate) {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				return candidate, nil
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("agent project directory not found")
}

func hasAgentProject(dir string) bool {
	if stat, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil || stat.IsDir() {
		return false
	}
	return hasAgentSource(dir)
}

func hasAgentSource(dir string) bool {
	stat, err := os.Stat(filepath.Join(dir, "cmd", "main.go"))
	return err == nil && !stat.IsDir()
}

func findGoExecutable() (string, error) {
	if goExe, err := exec.LookPath("go"); err == nil {
		return goExe, nil
	}
	candidates := []string{}
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		name := "go"
		if runtime.GOOS == "windows" {
			name = "go.exe"
		}
		candidates = append(candidates, filepath.Join(goroot, "bin", name))
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, `D:\Program Files\go\bin\go.exe`, `C:\Program Files\Go\bin\go.exe`)
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("go executable not found; cannot build agent binaries")
}

func buildAgentBinary(goExe, agentDir, output, goos, goarch string) error {
	cmd := exec.Command(goExe, "build", "-trimpath", "-o", output, "./cmd/main.go")
	cmd.Dir = agentDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+goos,
		"GOARCH="+goarch,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build %s/%s agent failed: %w\n%s", goos, goarch, err, string(out))
	}
	return nil
}

func findAgentBinary() (string, error) {
	candidates := []string{
		filepath.Join("agent", "bin", "agent-linux-amd64"),
		filepath.Join("..", "agent", "bin", "agent-linux-amd64"),
		filepath.Join("..", "..", "agent", "bin", "agent-linux-amd64"),
		filepath.Join("agent", "bin", "agent"),
		filepath.Join("..", "agent", "bin", "agent"),
		filepath.Join("..", "..", "agent", "bin", "agent"),
		filepath.Join("agent", "bin", "mysql-ops-agent-linux"),
		filepath.Join("..", "agent", "bin", "mysql-ops-agent-linux"),
		filepath.Join("..", "..", "agent", "bin", "mysql-ops-agent-linux"),
		filepath.Join("agent", "bin", "mysql-ops-agent-linux-amd64"),
		filepath.Join("..", "agent", "bin", "mysql-ops-agent-linux-amd64"),
		filepath.Join("agent", "bin", "agent_linux"),
		filepath.Join("..", "agent", "bin", "agent_linux"),
		filepath.Join("agent", "bin", "dbops-agent-linux"),
		filepath.Join("..", "agent", "bin", "dbops-agent-linux"),
	}
	for _, candidate := range candidates {
		ok, err := isLinuxELFFile(candidate)
		if err == nil && ok {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("valid Linux agent binary not found; restart backend to rebuild agent/bin/agent-linux-amd64")
}

func isLinuxELFFile(path string) (bool, error) {
	header, err := readHeader(path, 4)
	if err != nil {
		return false, err
	}
	return len(header) >= 4 && header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F', nil
}

func isWindowsPEFile(path string) (bool, error) {
	header, err := readHeader(path, 2)
	if err != nil {
		return false, err
	}
	return len(header) >= 2 && header[0] == 'M' && header[1] == 'Z', nil
}

func readHeader(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	read, err := f.Read(buf)
	if err != nil && read == 0 {
		return nil, err
	}
	return buf[:read], nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
