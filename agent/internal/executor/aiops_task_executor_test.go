package executor

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/monkeycode/mysql-ops-agent/internal/aioptimizer"
)

func TestFlavorTaskExecutorRunsAIDiagnoseWithoutBundle(t *testing.T) {
	root := t.TempDir()
	ran := map[string]bool{}
	executor := NewFlavorTaskExecutorWithPackageRoot(root, func(_ context.Context, name string, _ ...string) (string, error) {
		ran[name] = true
		switch name {
		case "df":
			return "Filesystem 1024-blocks Used Available Capacity Mounted\n/dev/sda1 100 95 5 95% /data", nil
		default:
			return "ok", nil
		}
	})
	req := FlavorTaskRequest{TaskID: "ai-1", InstanceID: "inst-1", Flavor: "dm", Version: "DM9", Operation: "diagnose", PackagePath: filepath.Join(root, "dm", "DM9"), TLS: &FlavorTLSConfig{}}

	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	for _, probe := range []string{"uptime", "df", "free", "ps"} {
		if !ran[probe] {
			t.Fatalf("probe %s was not executed, ran=%v", probe, ran)
		}
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok || data["flavor"] != "dm" {
		t.Fatalf("data = %#v", result.Data)
	}
	raw, _ := json.Marshal(data["diagnosis"])
	if !strings.Contains(string(raw), "evidence-only") {
		t.Fatalf("expected heuristic diagnosis summary, got: %s", string(raw))
	}
}

func TestFlavorTaskExecutorSmartFixRequiresLLMAndConsent(t *testing.T) {
	root := t.TempDir()
	executor := NewFlavorTaskExecutorWithPackageRoot(root, nil)
	base := FlavorTaskRequest{TaskID: "ai-2", InstanceID: "inst-1", Flavor: "tidb", Version: "v8.5.7", Operation: "smart-fix", PackagePath: filepath.Join(root, "tidb", "v8.5.7"), TLS: &FlavorTLSConfig{}}

	result, _ := executor.Execute(context.Background(), base)
	if result.Status != "failed" || !strings.Contains(result.Message, "ai.llm") {
		t.Fatalf("result = %#v", result)
	}

	withLLM := base
	withLLM.AI = &AIOpsConfig{LLM: &aioptimizer.LLMConfig{BaseURL: "http://127.0.0.1:1", Model: "m"}}
	result, _ = executor.Execute(context.Background(), withLLM)
	if result.Status != "failed" || !strings.Contains(result.Message, "confirm_apply") {
		t.Fatalf("result = %#v", result)
	}
}

func TestFlavorTaskExecutorAIOpsRunsHeuristicWhenLLMMissing(t *testing.T) {
	root := t.TempDir()
	executor := NewFlavorTaskExecutorWithPackageRoot(root, func(_ context.Context, name string, _ ...string) (string, error) {
		if name == "df" {
			return "Filesystem 1024-blocks Used Available Capacity Mounted\n/dev/sdb1 100 92 8 92% /data", nil
		}
		return "ok", nil
	})
	req := FlavorTaskRequest{TaskID: "ai-3", InstanceID: "inst-1", Flavor: "gbase8a", Version: "10.1", Operation: "diagnose", PackagePath: filepath.Join(root, "gbase8a", "10.1"), TLS: &FlavorTLSConfig{}}

	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	raw, _ := json.Marshal(result.Data)
	if !strings.Contains(string(raw), "heuristic") || !strings.Contains(string(raw), "/dev/sdb1") {
		t.Fatalf("diagnosis missing heuristic findings: %s", string(raw))
	}
}
