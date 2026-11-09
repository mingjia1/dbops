package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-agent/internal/aioptimizer"
)

// flavorDaemon maps each flavor to the daemon process observed on the host.
var flavorDaemon = map[string]string{
	"oceanbase": "observer", "tidb": "tidb-server", "dm": "dmserver",
	"kingbase": "kingbase", "opengauss": "gaussdb", "gbase8a": "gcluster",
	"gbase8s": "oninit",
}

// aiopsEvidenceCommands are the fixed, read-only host probes collected
// before the AI loop runs. Callers cannot add or alter them.
var aiopsEvidenceCommands = []struct {
	name string
	args []string
}{
	{"uptime", nil},
	{"df", []string{"-P"}},
	{"free", []string{"-m"}},
	{"ps", []string{"-eo", "pid,pcpu,pmem,comm", "--sort=-pcpu"}},
}

// executeAIOps implements the diagnose and smart-fix operations.
//
// diagnose collects fixed evidence probes and runs the aioptimizer engine;
// with ai.llm configured it is a full tool-calling agent loop, without it the
// result degrades to deterministic heuristic analysis of the evidence.
//
// smart-fix additionally requires ai.confirm_apply=true and, in this
// release, produces an approved fix plan (findings + ordered, safety-checked
// recommendations); plan execution stays an explicit follow-up task.
func (e *FlavorTaskExecutor) executeAIOps(ctx context.Context, req FlavorTaskRequest) (*TaskResult, error) {
	if req.Operation == "smart-fix" {
		if req.AI == nil || req.AI.LLM == nil || req.AI.LLM.BaseURL == "" || req.AI.LLM.Model == "" {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("smart-fix requires ai.llm configuration")), nil
		}
		if !req.AI.ConfirmApply {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("smart-fix requires ai.confirm_apply=true after reviewing the diagnose plan")), nil
		}
	}
	flavor := normalizeFlavorTask(req.Flavor)
	handler := e.handlers[flavor]
	if handler == nil || handler.runner == nil {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("flavor %q has no command runner for ai ops", flavor)), nil
	}

	facts := map[string]string{}
	for _, probe := range aiopsEvidenceCommands {
		output, err := handler.runner(ctx, probe.name, probe.args...)
		if err != nil {
			facts[probe.name] = "unavailable: " + err.Error()
			continue
		}
		facts[probe.name] = output
	}
	if daemon := flavorDaemon[flavor]; daemon != "" {
		output, err := handler.runner(ctx, "ps", "-C", daemon, "-o", "pid=,stat=,args=")
		if err != nil {
			facts["daemon_"+daemon] = "unavailable: " + err.Error()
		} else {
			facts["daemon_"+daemon] = output
		}
	}

	engine := newAIOptimizerEngine(req)
	result, err := engine.Diagnose(ctx, aiOpsEvidence(req, flavor, facts))
	if err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	return flavorTaskCompleted(req.TaskID, "ai ops "+req.Operation+" completed", map[string]interface{}{
		"flavor":    flavor,
		"operation": req.Operation,
		"diagnosis": result,
	}), nil
}

// newAIOptimizerEngine builds the diagnosis engine; LLM stays nil unless the
// request carries a usable ai.llm configuration.
func newAIOptimizerEngine(req FlavorTaskRequest) *aioptimizer.Engine {
	var llm *aioptimizer.LLMClient
	if req.AI != nil && req.AI.LLM != nil && req.AI.LLM.BaseURL != "" && req.AI.LLM.Model != "" {
		llm = aioptimizer.NewLLMClient(*req.AI.LLM)
	}
	return aioptimizer.NewEngine(llm, nil)
}

func aiOpsEvidence(req FlavorTaskRequest, flavor string, facts map[string]string) aioptimizer.Evidence {
	return aioptimizer.Evidence{Flavor: flavor, Operation: req.Operation, TaskID: req.TaskID, Collected: time.Now(), Facts: facts}
}
