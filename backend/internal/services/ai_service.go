package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/aiprovider"
)

type AIService struct {
	provider   aiprovider.Provider
	diagnosisRepo *repositories.DiagnosisRepository
	adviceRepo    *repositories.SQLAdviceRepository
	monitorService *MonitorService
}

func NewAIService(provider aiprovider.Provider, diagnosisRepo *repositories.DiagnosisRepository, adviceRepo *repositories.SQLAdviceRepository, monitorService *MonitorService) *AIService {
	return &AIService{
		provider:      provider,
		diagnosisRepo: diagnosisRepo,
		adviceRepo:    adviceRepo,
		monitorService: monitorService,
	}
}

// Chat answers an operations question through the configured AI provider.
// History is clamped to the last 20 messages to bound token usage.
func (s *AIService) Chat(ctx context.Context, messages []aiprovider.Message) (*aiprovider.ChatResponse, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("AI provider is not configured")
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("at least one message is required")
	}
	if len(messages) > 20 {
		messages = messages[len(messages)-20:]
	}
	chatReq := aiprovider.ChatRequest{
		Messages: append([]aiprovider.Message{{Role: "system", Content: OpsAssistantSystemPrompt}}, messages...),
	}
	return s.provider.Chat(ctx, chatReq)
}

// OpsAssistantSystemPrompt grounds the AI chat panel in DBA operations.
const OpsAssistantSystemPrompt = `You are a senior database operations assistant for a multi-flavor DB ops platform (MySQL, Dameng, Kingbase, openGauss, GBase 8a/8s, OceanBase, TiDB).
Answer in the user's language. Be concise and concrete: give ordered, actionable steps with commands where possible.
Never propose destructive operations (drop, truncate, rm, force-uninstall) without a verified backup and explicit human approval.`

// Diagnosis runs AI diagnosis for an instance.
func (s *AIService) Diagnosis(ctx context.Context, instanceID string) (*models.DiagnosisRecord, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("AI provider is not configured")
	}

	// Gather instance data
	// 与巡检同类修复: agent 实际采集集为 MySQL STATUS 指标, 不含 cpu/mem/disk
	// (实例级系统资源指标全平台无采集源), 请求它们只会得到空行; 改为请求真实
	// 可得指标, 查询失败时在诊断上下文中显式声明, 不静默吞错。
	diagnosisMetrics := []string{
		"qps", "tps", "connections", "threads_connected", "threads_running",
		"slow_queries", "uptime", "innodb_buffer_pool_hit_ratio",
	}
	var metrics []MetricData
	var metricsErr error
	if s.monitorService == nil {
		metricsErr = fmt.Errorf("monitor service not available")
	} else {
		metrics, metricsErr = s.monitorService.QueryMetrics(ctx, MetricQueryRequest{
			InstanceID: instanceID,
			Metrics:    diagnosisMetrics,
		})
	}

	metricLines := make([]string, 0, len(metrics))
	for _, m := range metrics {
		metricLines = append(metricLines, fmt.Sprintf("  %s: %.2f", m.Name, m.Value))
	}
	if metricsErr != nil {
		metricLines = append(metricLines, "  (监控指标查询失败: "+metricsErr.Error()+")")
	} else if len(metricLines) == 0 {
		metricLines = append(metricLines, "  (暂无可用监控指标)")
	}

	prompt := strings.ReplaceAll(aiprovider.PromptTemplates.Diagnosis, "{{INSTANCE_INFO}}", fmt.Sprintf("Instance ID: %s", instanceID))
	prompt = strings.ReplaceAll(prompt, "{{METRICS}}", strings.Join(metricLines, "\n"))
	prompt = strings.ReplaceAll(prompt, "{{ALERTS}}", "(no active alerts — data not available in this context)")
	prompt = strings.ReplaceAll(prompt, "{{CONFIG}}", "(see metrics above)")

	chatReq := aiprovider.ChatRequest{
		Messages: []aiprovider.Message{
			{Role: "system", Content: "You are a MySQL DBA expert. Respond in JSON format with fields: status, issues (array), recommendations (array), risk_level."},
			{Role: "user", Content: prompt},
		},
	}

	resp, err := s.provider.Chat(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("ai diagnosis failed: %w", err)
	}

	record := &models.DiagnosisRecord{
		InstanceID: instanceID,
		Status:     "completed",
		Summary:    resp.Content,
		Details:    fmt.Sprintf(`{"model":"%s","usage":{"prompt":%d,"completion":%d,"total":%d}}`, resp.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens),
		Score:      0,
		CreatedAt:  time.Now(),
	}

	// Parse JSON response for score
	var parsed struct {
		Status  string `json:"status"`
		Issues  []interface{} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err == nil {
		switch parsed.Status {
		case "healthy":
			record.Score = 100
		case "warning":
			record.Score = 60
		default:
			record.Score = 30
		}
	}

	if err := s.diagnosisRepo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to save diagnosis: %w", err)
	}

	return record, nil
}

// SQLAdvice analyzes SQL with AI.
func (s *AIService) SQLAdvice(ctx context.Context, sqlText, explainPlan, schema string) (*models.SQLAdvice, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("AI provider is not configured")
	}

	prompt := strings.ReplaceAll(aiprovider.PromptTemplates.SQLAdvisor, "{{SQL}}", sqlText)
	prompt = strings.ReplaceAll(prompt, "{{EXPLAIN}}", explainPlan)
	prompt = strings.ReplaceAll(prompt, "{{SCHEMA}}", schema)

	chatReq := aiprovider.ChatRequest{
		Messages: []aiprovider.Message{
			{Role: "system", Content: "You are a MySQL SQL optimization expert. Respond in JSON format with fields: analysis, issues (array), optimized_sql (optional), index_recommendations (array), score (0-100)."},
			{Role: "user", Content: prompt},
		},
	}

	resp, err := s.provider.Chat(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("ai sql advisor failed: %w", err)
	}

	var parsed struct {
		Score int `json:"score"`
	}
	score := 0
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err == nil {
		score = parsed.Score
	}

	advice := &models.SQLAdvice{
		SQLText:   sqlText,
		Explain:   explainPlan,
		Advice:    resp.Content,
		Score:     score,
		CreatedAt: time.Now(),
	}

	if err := s.adviceRepo.Create(ctx, advice); err != nil {
		return nil, fmt.Errorf("failed to save sql advice: %w", err)
	}

	return advice, nil
}

// ListDiagnoses returns diagnosis history for an instance.
func (s *AIService) ListDiagnoses(ctx context.Context, instanceID string, limit, offset int) ([]models.DiagnosisRecord, error) {
	return s.diagnosisRepo.List(ctx, instanceID, limit, offset)
}

// GetDiagnosis returns a single diagnosis record.
func (s *AIService) GetDiagnosis(ctx context.Context, id string) (*models.DiagnosisRecord, error) {
	return s.diagnosisRepo.GetByID(ctx, id)
}

// ListSQLAdvice returns SQL advice history.
func (s *AIService) ListSQLAdvice(ctx context.Context, limit, offset int) ([]models.SQLAdvice, error) {
	return s.adviceRepo.List(ctx, limit, offset)
}

// GetSQLAdvice returns a single SQL advice record.
func (s *AIService) GetSQLAdvice(ctx context.Context, id string) (*models.SQLAdvice, error) {
	return s.adviceRepo.GetByID(ctx, id)
}

func (s *AIService) Provider() aiprovider.Provider {
	return s.provider
}
