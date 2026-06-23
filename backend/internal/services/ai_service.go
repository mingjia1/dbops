package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/aiprovider"
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

// Diagnosis runs AI diagnosis for an instance.
func (s *AIService) Diagnosis(ctx context.Context, instanceID string) (*models.DiagnosisRecord, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("AI provider is not configured")
	}

	// Gather instance data
	metrics, _ := s.monitorService.QueryMetrics(ctx, MetricQueryRequest{
		InstanceID: instanceID,
		Metrics:    []string{"cpu", "mem", "disk", "connections", "qps", "threads_running", "innodb_deadlocks"},
	})

	metricLines := make([]string, 0, len(metrics))
	for _, m := range metrics {
		metricLines = append(metricLines, fmt.Sprintf("  %s: %.2f", m.Name, m.Value))
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
