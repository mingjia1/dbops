package aioptimizer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Evidence is the pre-collected observation set handed to the engine.
// Facts are fixed read-only probes; the model may request more via tools.
type Evidence struct {
	Flavor    string            `json:"flavor"`
	Operation string            `json:"operation"`
	TaskID    string            `json:"task_id"`
	Collected time.Time         `json:"collected"`
	Facts     map[string]string `json:"facts"`
}

// Result is the engine output. Mode is "llm" (full agent loop) or
// "heuristic" (deterministic evidence analysis, no AI provider).
type Result struct {
	Mode            string   `json:"mode"`
	Summary         string   `json:"summary"`
	Findings        []string `json:"findings"`
	Recommendations []string `json:"recommendations"`
	ToolCalls       []string `json:"tool_calls,omitempty"`
}

// Engine runs bounded agentic diagnosis over the tool registry.
type Engine struct {
	llm           *LLMClient
	registry      *Registry
	maxIterations int
}

func NewEngine(llm *LLMClient, registry *Registry) *Engine {
	return &Engine{llm: llm, registry: registry, maxIterations: 6}
}

const diagnoseSystemPrompt = `You are a senior database reliability engineer for a multi-flavor DB ops platform (MySQL-compatible, Dameng, Kingbase, openGauss, GBase 8a/8s, OceanBase, TiDB).
You diagnose a database host from collected evidence. You may call the provided read-only tools to gather more data, but at most a few times.
Finally respond with ONLY a JSON object: {"summary": string, "findings": [string], "recommendations": [string]}.
Findings must reference concrete evidence. Recommendations must be safe, ordered actions; destructive operations require a verified backup and human approval.`

// Diagnose runs one diagnosis. Without an LLM it degrades to deterministic
// evidence analysis so the operation stays useful in air-gapped installs.
func (e *Engine) Diagnose(ctx context.Context, evidence Evidence) (*Result, error) {
	if evidence.Collected.IsZero() {
		evidence.Collected = time.Now()
	}
	if e.llm == nil {
		return heuristicDiagnosis(evidence), nil
	}
	return e.llmDiagnose(ctx, evidence)
}

func (e *Engine) llmDiagnose(ctx context.Context, evidence Evidence) (*Result, error) {
	evidenceJSON, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("aioptimizer: marshal evidence: %w", err)
	}
	messages := []ChatMessage{
		{Role: "system", Content: diagnoseSystemPrompt},
		{Role: "user", Content: "Diagnose this host and reply with the JSON object.\n" + string(evidenceJSON)},
	}

	var toolCalls []string
	for iteration := 0; iteration < e.maxIterations; iteration++ {
		message, err := e.llm.Chat(ctx, messages, e.registry.Definitions())
		if err != nil {
			return nil, err
		}
		if len(message.ToolCalls) == 0 {
			return parseLLMResult(message.Content, toolCalls)
		}
		messages = append(messages, *message)
		for _, call := range message.ToolCalls {
			toolCalls = append(toolCalls, call.Function.Name)
			messages = append(messages, e.invokeTool(ctx, call))
		}
	}
	return nil, fmt.Errorf("aioptimizer: diagnosis did not converge within %d iterations", e.maxIterations)
}

func (e *Engine) invokeTool(ctx context.Context, call ToolCall) ChatMessage {
	tool, ok := e.registry.Get(call.Function.Name)
	if !ok {
		return ChatMessage{Role: "tool", ToolCallID: call.ID, Content: "error: unknown tool " + call.Function.Name}
	}
	output, err := tool.Invoke(ctx, json.RawMessage(call.Function.Arguments))
	if err != nil {
		output = "error: " + err.Error()
	}
	if len(output) > 4000 {
		output = output[:4000] + "...(truncated)"
	}
	return ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: output}
}

var (
	diskUsageLine  = regexp.MustCompile(`(?m)^(\S+)\s+\d+\s+\d+\s+\d+\s+(\d+)%\s+(\S+)$`)
	heuristicNoise = regexp.MustCompile(`(?m)^(Filesystem|tmpfs|devtmpfs|overlay|shm)\b`)
)

// heuristicDiagnosis performs deterministic checks on the collected facts:
// flagged filesystems (>=90% used) and unavailable probes.
func heuristicDiagnosis(evidence Evidence) *Result {
	result := &Result{Mode: "heuristic", Findings: []string{}, Recommendations: []string{}}
	for name, fact := range evidence.Facts {
		if strings.HasPrefix(fact, "unavailable:") {
			result.Findings = append(result.Findings, fmt.Sprintf("probe %q failed: %s", name, fact))
			continue
		}
		if name != "df" {
			continue
		}
		for _, match := range diskUsageLine.FindAllStringSubmatch(fact, -1) {
			filesystem, usedPct, mount := match[1], match[2], match[3]
			if heuristicNoise.MatchString(filesystem) {
				continue
			}
			pct, err := strconv.Atoi(strings.TrimSuffix(usedPct, "%"))
			if err == nil && pct >= 90 {
				result.Findings = append(result.Findings, fmt.Sprintf("filesystem %s on %s is %s%% full", filesystem, mount, usedPct))
			}
		}
	}
	if len(result.Findings) == 0 {
		result.Findings = append(result.Findings, "no automatic findings in the collected evidence")
	}
	result.Recommendations = append(result.Recommendations,
		"configure ai.llm on the task request to enable full AI diagnosis with tool access",
		"attach this evidence to the task so a DBA can review it")
	result.Summary = fmt.Sprintf("evidence-only diagnosis for %s/%s: %d probes collected", evidence.Flavor, evidence.Operation, len(evidence.Facts))
	return result
}

func parseLLMResult(content string, toolCalls []string) (*Result, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var parsed struct {
		Summary         string   `json:"summary"`
		Findings        []string `json:"findings"`
		Recommendations []string `json:"recommendations"`
	}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		// Model skipped JSON: keep the raw text instead of failing the task.
		return &Result{Mode: "llm", Summary: content, Findings: []string{}, Recommendations: []string{}, ToolCalls: toolCalls}, nil
	}
	return &Result{Mode: "llm", Summary: parsed.Summary, Findings: parsed.Findings, Recommendations: parsed.Recommendations, ToolCalls: toolCalls}, nil
}
