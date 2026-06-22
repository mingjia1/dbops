package executor

import (
	"context"
	"fmt"
)

type NodeRebuild struct{}

func NewNodeRebuild() *NodeRebuild {
	return &NodeRebuild{}
}

type RebuildNodeConfig struct {
	Port       int    `json:"port"`
	DataDir    string `json:"data_dir"`
	Basedir    string `json:"basedir"`
	OSUser     string `json:"os_user"`
	Flavor     string `json:"flavor"`
}

func (r *NodeRebuild) StopInstance(ctx context.Context, port int) error {
	_, err := runSystemctl(ctx, "stop", fmt.Sprintf("mysqld_%d", port))
	return err
}

func (r *NodeRebuild) ClearDataDir(ctx context.Context, dataDir string) error {
	if dataDir == "" {
		return fmt.Errorf("data_dir is required")
	}
	_, err := runShell(ctx, fmt.Sprintf("rm -rf %s/*", dataDir))
	return err
}

func (r *NodeRebuild) ReinitializeDataDir(ctx context.Context, basedir, dataDir, osUser string) error {
	if basedir == "" || dataDir == "" {
		return fmt.Errorf("basedir and data_dir are required")
	}
	binaryName := "mysqld"
	mysqld := fmt.Sprintf("%s/bin/%s", basedir, binaryName)
	_, err := runShell(ctx, fmt.Sprintf("%s --initialize-insecure --datadir=%s --user=%s", mysqld, dataDir, osUser))
	return err
}

func runSystemctl(ctx context.Context, action, service string) (string, error) {
	return runShell(ctx, fmt.Sprintf("systemctl %s %s", action, service))
}

func runShell(ctx context.Context, cmd string) (string, error) {
	return "", fmt.Errorf("shell execution not available in test: %s", cmd)
}
