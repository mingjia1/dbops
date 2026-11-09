package aioptimizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMConfig points the optimizer at an OpenAI-compatible chat API
// (DashScope/Qwen, vLLM, one-api gateways, Grok, Claude-compatible proxies).
type LLMConfig struct {
	BaseURL string        `json:"base_url"`
	APIKey  string        `json:"api_key"`
	Model   string        `json:"model"`
	Timeout time.Duration `json:"-"`
}

// ChatMessage is one message in a conversation; ToolCalls appear on
// assistant messages and ToolCallID marks tool result messages.
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolDefinition is the wire format for advertising a registry tool.
type ToolDefinition struct {
	Type     string           `json:"type"`
	Function ToolDefinitionFn `json:"function"`
}

type ToolDefinitionFn struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// LLMClient is a minimal OpenAI-compatible chat client with tool calling.
type LLMClient struct {
	config LLMConfig
	client *http.Client
}

func NewLLMClient(config LLMConfig) *LLMClient {
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}
	return &LLMClient{config: config, client: &http.Client{Timeout: config.Timeout}}
}

// Chat sends one completion request; toolDefs may be nil for plain chat.
func (c *LLMClient) Chat(ctx context.Context, messages []ChatMessage, toolDefs []ToolDefinition) (*ChatMessage, error) {
	body := map[string]interface{}{
		"model":       c.config.Model,
		"messages":    messages,
		"temperature": 0.2,
	}
	if len(toolDefs) > 0 {
		body["tools"] = toolDefs
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("aioptimizer: marshal request: %w", err)
	}

	baseURL := strings.TrimRight(c.config.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("aioptimizer: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("aioptimizer: http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("aioptimizer: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aioptimizer: API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("aioptimizer: parse response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("aioptimizer: empty choices")
	}
	return &parsed.Choices[0].Message, nil
}
