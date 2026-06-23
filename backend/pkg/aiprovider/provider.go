package aiprovider

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

// Provider defines the AI provider interface.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// Config holds AI provider configuration.
type Config struct {
	BaseURL    string  `json:"base_url"`
	APIKey     string  `json:"api_key"`
	Model      string  `json:"model"`
	MaxTokens  int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

type ChatRequest struct {
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	Usage     Usage  `json:"usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIProvider implements Provider for OpenAI-compatible APIs.
type OpenAIProvider struct {
	config Config
	client *http.Client
}

func NewOpenAIProvider(config Config) *OpenAIProvider {
	return &OpenAIProvider{
		config: config,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body := map[string]interface{}{
		"model":       p.config.Model,
		"messages":    req.Messages,
		"max_tokens":  p.config.MaxTokens,
		"temperature": p.config.Temperature,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("aiprovider: marshal request: %w", err)
	}

		baseURL := strings.TrimRight(p.config.BaseURL, "/")
		if !strings.HasSuffix(baseURL, "/v1") {
			baseURL += "/v1"
		}
		httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("aiprovider: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("aiprovider: http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("aiprovider: read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("aiprovider: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage Usage  `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("aiprovider: parse response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("aiprovider: empty choices")
	}

	return &ChatResponse{
		Content: openAIResp.Choices[0].Message.Content,
		Model:   openAIResp.Model,
		Usage:   openAIResp.Usage,
	}, nil
}

// PromptTemplates defines prompt strings for different AI use cases.
var PromptTemplates = struct {
	Diagnosis    string
	SQLAdvisor   string
	AnomalyExplain string
}{
	Diagnosis: `You are a MySQL DBA expert. Analyze the following instance metrics and alert information to provide a diagnosis.

Instance Info:
{{INSTANCE_INFO}}

Metrics (last 15 min):
{{METRICS}}

Active Alerts:
{{ALERTS}}

Configuration:
{{CONFIG}}

Please provide:
1. Overall health status (healthy/warning/critical)
2. Key issues found
3. Recommended actions (prioritized)
4. Risk assessment`,

	SQLAdvisor: `You are a MySQL SQL optimization expert. Analyze the following SQL query and its EXPLAIN plan to provide optimization suggestions.

SQL:
{{SQL}}

EXPLAIN:
{{EXPLAIN}}

Table Schema:
{{SCHEMA}}

Please provide:
1. Performance analysis
2. Identified issues (scan type, missing indexes, etc.)
3. Optimized SQL (if applicable)
4. Index recommendations`,

	AnomalyExplain: `You are a MySQL DBA expert. Analyze the following anomaly data and suggest possible root causes.

Anomaly:
{{ANOMALY}}

Metrics Timeline:
{{METRICS}}

Recent Changes:
{{CHANGES}}

Please provide:
1. Likely root cause
2. Confidence level (high/medium/low)
3. Verification steps
4. Resolution plan`,
}
