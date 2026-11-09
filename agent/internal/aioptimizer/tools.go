package aioptimizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Tool mirrors the MCP tool contract: a name, a JSON schema for its input,
// and a handler. The registry is the toolbox an AI agent is allowed to call;
// everything outside it is unreachable from the model.
type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Invoke      func(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry is a name-indexed tool set.
type Registry struct {
	tools map[string]*Tool
	order []string
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

// Register adds a tool; empty or duplicate names are rejected.
func (r *Registry) Register(tool *Tool) error {
	if tool == nil || strings.TrimSpace(tool.Name) == "" {
		return fmt.Errorf("aioptimizer: tool name is required")
	}
	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("aioptimizer: tool %q already registered", tool.Name)
	}
	r.tools[tool.Name] = tool
	r.order = append(r.order, tool.Name)
	return nil
}

func (r *Registry) Get(name string) (*Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) List() []*Tool {
	out := make([]*Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// Definitions renders the registry as OpenAI-style tool definitions.
func (r *Registry) Definitions() []ToolDefinition {
	out := make([]ToolDefinition, 0, len(r.order))
	for _, tool := range r.List() {
		out = append(out, ToolDefinition{
			Type: "function",
			Function: ToolDefinitionFn{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Schema,
			},
		})
	}
	return out
}

// diagnosticCommandAllowlist restricts the agent loop to read-only host
// probes. Nothing outside this set can be executed through the tool.
var diagnosticCommandAllowlist = map[string]bool{
	"uptime": true, "df": true, "free": true, "ps": true, "top": true,
	"iostat": true, "vmstat": true, "ss": true, "netstat": true,
	"sar": true, "lscpu": true, "who": true,
}

var runHostCommandSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {"type": "string", "description": "read-only diagnostic command"},
    "args": {"type": "array", "items": {"type": "string"}, "description": "command arguments"}
  },
  "required": ["command"]
}`)

// NewDiagnosticRegistry wires the read-only host evidence tools that an AI
// loop may call. The runner never goes through a shell, and both the binary
// and its arguments are validated against the allowlist and sanitation rules.
func NewDiagnosticRegistry(runner func(ctx context.Context, name string, args ...string) (string, error)) *Registry {
	registry := NewRegistry()
	_ = registry.Register(&Tool{
		Name:        "run_host_command",
		Description: "Run a read-only diagnostic command on the database host (uptime, df, free, ps, top, iostat, vmstat, ss, netstat, sar, lscpu, who).",
		Schema:      runHostCommandSchema,
		Invoke: func(ctx context.Context, args json.RawMessage) (string, error) {
			var parsed struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			}
			if err := json.Unmarshal(args, &parsed); err != nil {
				return "", fmt.Errorf("parse tool arguments: %w", err)
			}
			if !diagnosticCommandAllowlist[parsed.Command] {
				return "", fmt.Errorf("command %q is not in the diagnostic allowlist", parsed.Command)
			}
			for _, arg := range parsed.Args {
				if arg == "" || strings.ContainsAny(arg, "\n\r\x00;|&`$><\\") || strings.Contains(arg, "..") {
					return "", fmt.Errorf("argument %q is not allowed", arg)
				}
			}
			if runner == nil {
				return "", fmt.Errorf("command runner is not configured")
			}
			return runner(ctx, parsed.Command, parsed.Args...)
		},
	})
	return registry
}
