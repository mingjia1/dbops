package kernel

import (
	"context"
	"fmt"
	"strings"
)

// AgentTaskCaller invokes one Agent task and returns its task result payload.
// A completed Agent task is required for every lifecycle phase to succeed.
type AgentTaskCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)

// LifecycleOperation names the database lifecycle action delegated to an Agent.
type LifecycleOperation string

const (
	OperationDeploy      LifecycleOperation = "deploy"
	OperationConfigure   LifecycleOperation = "configure"
	OperationBackup      LifecycleOperation = "backup"
	OperationRestore     LifecycleOperation = "restore"
	OperationUpgrade     LifecycleOperation = "upgrade"
	OperationMigrate     LifecycleOperation = "migrate"
	OperationHA          LifecycleOperation = "ha"
	OperationReplication LifecycleOperation = "replication"
	OperationFailover    LifecycleOperation = "failover"
	OperationMonitor     LifecycleOperation = "monitor"
)

// XinchuangCorePlugin is the common lifecycle contract for dedicated 信创
// flavor executors. Every method issues an Agent task and accepts only a
// terminal successful result from that Agent.
type XinchuangCorePlugin interface {
	Name() string
	Type() string
	Version() string
	Flavor() string
	Prepare(ctx context.Context, env KernelEnv) error
	Execute(ctx context.Context, env KernelEnv, params map[string]interface{}) (*KernelResult, error)
	Rollback(ctx context.Context, env KernelEnv) error
	Teardown(ctx context.Context, env KernelEnv) error
	Join(ctx context.Context, env KernelEnv, newNode KernelNode) error
	Leave(ctx context.Context, env KernelEnv, node KernelNode) error
}

// XinchuangCoreBase supplies the shared Agent task behavior used by individual
// flavor executors. taskPath is intentionally injected so each flavor can bind
// its own Agent handler without changing the lifecycle contract.
type XinchuangCoreBase struct {
	flavor      string
	taskPath    string
	agentCaller AgentTaskCaller
}

func NewXinchuangCoreBase(flavor, taskPath string, agentCaller AgentTaskCaller) *XinchuangCoreBase {
	return &XinchuangCoreBase{
		flavor:      strings.ToLower(strings.TrimSpace(flavor)),
		taskPath:    strings.TrimSpace(taskPath),
		agentCaller: agentCaller,
	}
}

func (p *XinchuangCoreBase) Name() string    { return p.flavor + "-core" }
func (p *XinchuangCoreBase) Type() string    { return "kernel" }
func (p *XinchuangCoreBase) Version() string { return "1.0.0" }
func (p *XinchuangCoreBase) Flavor() string  { return p.flavor }

func (p *XinchuangCoreBase) Prepare(_ context.Context, env KernelEnv) error {
	if p.flavor == "" {
		return fmt.Errorf("flavor is empty")
	}
	if p.taskPath == "" {
		return fmt.Errorf("agent task path is empty")
	}
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	if env.Node.Address == "" {
		return fmt.Errorf("node address is empty")
	}
	if env.Node.AgentPort == 0 {
		return fmt.Errorf("node agent port is zero")
	}
	return nil
}

func (p *XinchuangCoreBase) Execute(ctx context.Context, env KernelEnv, params map[string]interface{}) (*KernelResult, error) {
	operation, err := lifecycleOperation(params)
	if err != nil {
		return nil, err
	}
	response, err := p.runAgentTask(ctx, env, operation, params)
	if err != nil {
		return nil, err
	}
	return &KernelResult{
		Success: true,
		Message: agentTaskMessage(response, string(operation)+" completed"),
		Basedir: env.Node.Basedir,
		Datadir: env.Node.DataDir,
		Data:    response,
	}, nil
}

func (p *XinchuangCoreBase) Rollback(ctx context.Context, env KernelEnv) error {
	_, err := p.runAgentTask(ctx, env, "rollback", nil)
	return err
}

func (p *XinchuangCoreBase) Teardown(ctx context.Context, env KernelEnv) error {
	_, err := p.runAgentTask(ctx, env, "teardown", nil)
	return err
}

func (p *XinchuangCoreBase) Join(ctx context.Context, env KernelEnv, newNode KernelNode) error {
	if newNode.Address == "" || newNode.AgentPort == 0 {
		return fmt.Errorf("joining node address and agent port are required")
	}
	_, err := p.runAgentTask(ctx, env, "join", map[string]interface{}{"node": newNode})
	return err
}

func (p *XinchuangCoreBase) Leave(ctx context.Context, env KernelEnv, node KernelNode) error {
	if node.Address == "" || node.AgentPort == 0 {
		return fmt.Errorf("leaving node address and agent port are required")
	}
	_, err := p.runAgentTask(ctx, env, "leave", map[string]interface{}{"node": node})
	return err
}

func (p *XinchuangCoreBase) runAgentTask(ctx context.Context, env KernelEnv, operation LifecycleOperation, params map[string]interface{}) (map[string]interface{}, error) {
	if err := p.Prepare(ctx, env); err != nil {
		return nil, err
	}
	payload := make(map[string]interface{}, len(params)+4)
	for key, value := range params {
		payload[key] = value
	}
	payload["task_id"] = fmt.Sprintf("%s-%s-%s", p.flavor, operation, env.ClusterID)
	payload["flavor"] = p.flavor
	payload["operation"] = string(operation)
	payload["target"] = env.Node

	response, err := p.agentCaller(ctx, env.Node.Address, env.Node.AgentPort, p.taskPath, payload)
	if err != nil {
		return nil, fmt.Errorf("agent %s task: %w", operation, err)
	}
	if err := ensureAgentTaskSucceeded(p.flavor, operation, response); err != nil {
		return nil, err
	}
	return response, nil
}

func lifecycleOperation(params map[string]interface{}) (LifecycleOperation, error) {
	if params == nil {
		return "", fmt.Errorf("lifecycle operation is required")
	}
	raw, ok := params["operation"]
	if !ok {
		return "", fmt.Errorf("lifecycle operation is required")
	}
	operation := LifecycleOperation(strings.ToLower(strings.TrimSpace(fmt.Sprint(raw))))
	switch operation {
	case OperationDeploy, OperationConfigure, OperationBackup, OperationRestore, OperationUpgrade,
		OperationMigrate, OperationHA, OperationReplication, OperationFailover, OperationMonitor:
		return operation, nil
	default:
		return "", fmt.Errorf("unsupported lifecycle operation %q", operation)
	}
}

func ensureAgentTaskSucceeded(flavor string, operation LifecycleOperation, response map[string]interface{}) error {
	if response == nil {
		return fmt.Errorf("%s %s returned an empty agent result", flavor, operation)
	}
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(response["status"])))
	switch status {
	case "completed", "success", "succeeded", "ok":
		return nil
	case "":
		return fmt.Errorf("%s %s returned a missing agent task status", flavor, operation)
	default:
		return fmt.Errorf("%s %s returned agent task status %q: %s", flavor, operation, status, agentTaskMessage(response, "no message"))
	}
}

func agentTaskMessage(response map[string]interface{}, fallback string) string {
	message := strings.TrimSpace(fmt.Sprint(response["message"]))
	if message == "" || message == "<nil>" {
		return fallback
	}
	return message
}
