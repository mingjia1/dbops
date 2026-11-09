package aioptimizer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryRejectsEmptyAndDuplicateNames(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(&Tool{Name: " ", Invoke: func(context.Context, json.RawMessage) (string, error) { return "", nil }}); err == nil {
		t.Fatal("expected empty name to be rejected")
	}
	if err := registry.Register(&Tool{Name: "probe", Invoke: func(context.Context, json.RawMessage) (string, error) { return "", nil }}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := registry.Register(&Tool{Name: "probe", Invoke: func(context.Context, json.RawMessage) (string, error) { return "", nil }}); err == nil {
		t.Fatal("expected duplicate name to be rejected")
	}
	if len(registry.List()) != 1 || len(registry.Definitions()) != 1 {
		t.Fatalf("registry size = %d", len(registry.List()))
	}
}

func TestDiagnosticRegistryAllowlist(t *testing.T) {
	var executed string
	runner := func(_ context.Context, name string, args ...string) (string, error) {
		executed = name + " " + strings.Join(args, " ")
		return "ok", nil
	}
	registry := NewDiagnosticRegistry(runner)
	tool, ok := registry.Get("run_host_command")
	if !ok {
		t.Fatal("run_host_command not registered")
	}

	if _, err := tool.Invoke(context.Background(), json.RawMessage(`{"command":"rm","args":["-rf","/"]}`)); err == nil {
		t.Fatal("expected non-allowlisted command to be rejected")
	}
	if executed != "" {
		t.Fatalf("rejected command was executed: %s", executed)
	}

	if _, err := tool.Invoke(context.Background(), json.RawMessage(`{"command":"df","args":["-P;reboot"]}`)); err == nil {
		t.Fatal("expected injected argument to be rejected")
	}

	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"command":"df","args":["-P"]}`))
	if err != nil || out != "ok" || executed != "df -P" {
		t.Fatalf("out=%q err=%v executed=%q", out, err, executed)
	}
}

func TestEngineHeuristicModeFlagsFullFilesystems(t *testing.T) {
	engine := NewEngine(nil, NewDiagnosticRegistry(nil))
	result, err := engine.Diagnose(context.Background(), Evidence{
		Flavor: "dm", Operation: "diagnose", TaskID: "t1",
		Facts: map[string]string{
			"df":    "Filesystem 1024-blocks Used Available Capacity Mounted\n/dev/sda1 100 95 5 95% /data\ntmpfs 10 1 9 10% /tmp",
			"free":  "unavailable: exit status 1",
		},
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if result.Mode != "heuristic" {
		t.Fatalf("mode = %q", result.Mode)
	}
	joined := strings.Join(result.Findings, "\n")
	if !strings.Contains(joined, "/dev/sda1") || !strings.Contains(joined, "95%") {
		t.Fatalf("expected 95%% filesystem flagged, got: %s", joined)
	}
	if strings.Contains(joined, "tmpfs") {
		t.Fatalf("tmpfs should be ignored: %s", joined)
	}
	if !strings.Contains(joined, "free") {
		t.Fatalf("expected failed probe reported: %s", joined)
	}
}

func TestEngineLLMToolLoop(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"run_host_command","arguments":"{\"command\":\"uptime\"}"}}]}}]}`))
		case 2:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"host healthy\",\"findings\":[\"load ok\"],\"recommendations\":[\"none\"]}"}}]}`))
		default:
			t.Fatalf("unexpected extra LLM call #%d", calls)
		}
	}))
	defer server.Close()

	var gotCommand string
	registry := NewDiagnosticRegistry(func(_ context.Context, name string, args ...string) (string, error) {
		gotCommand = name
		return " 14:00 up 10 days", nil
	})
	engine := NewEngine(NewLLMClient(LLMConfig{BaseURL: server.URL, Model: "test-model"}), registry)
	result, err := engine.Diagnose(context.Background(), Evidence{Flavor: "tidb", Operation: "diagnose", TaskID: "t2", Facts: map[string]string{"df": "ok"}})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if result.Mode != "llm" || result.Summary != "host healthy" {
		t.Fatalf("result = %#v", result)
	}
	if gotCommand != "uptime" {
		t.Fatalf("tool was not executed, gotCommand=%q", gotCommand)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0] != "run_host_command" {
		t.Fatalf("tool calls = %v", result.ToolCalls)
	}
}
