package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/aiprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAIProvider struct {
	response string
	err      error
}

func (m *mockAIProvider) Chat(ctx context.Context, req aiprovider.ChatRequest) (*aiprovider.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &aiprovider.ChatResponse{
		Content: m.response,
		Usage: aiprovider.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

func newTestAIService(t *testing.T, provider aiprovider.Provider) *AIService {
	db := newTestDB(t)
	monitorSvc := NewMonitorService(nil)
	return NewAIService(
		provider,
		repositories.NewDiagnosisRepository(db),
		repositories.NewSQLAdviceRepository(db),
		monitorSvc,
	)
}

func TestAIServiceDiagnosisHealthy(t *testing.T) {
	provider := &mockAIProvider{
		response: `{"status":"healthy","summary":"All checks passed","score":100}`,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	record, err := svc.Diagnosis(ctx, "instance-1")
	require.NoError(t, err)
	assert.Equal(t, 100, record.Score)
	assert.Equal(t, "instance-1", record.InstanceID)
	assert.NotEmpty(t, record.Details)
}

func TestAIServiceDiagnosisWarning(t *testing.T) {
	provider := &mockAIProvider{
		response: `{"status":"warning","summary":"High CPU usage detected","score":60}`,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	record, err := svc.Diagnosis(ctx, "instance-1")
	require.NoError(t, err)
	assert.Equal(t, 60, record.Score)
}

func TestAIServiceDiagnosisCritical(t *testing.T) {
	provider := &mockAIProvider{
		response: `{"status":"critical","summary":"Disk failure imminent","score":30}`,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	record, err := svc.Diagnosis(ctx, "instance-1")
	require.NoError(t, err)
	assert.Equal(t, 30, record.Score)
}

func TestAIServiceDiagnosisProviderError(t *testing.T) {
	provider := &mockAIProvider{
		err: assert.AnError,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	_, err := svc.Diagnosis(ctx, "instance-1")
	require.Error(t, err)
}

func TestAIServiceSQLAdvice(t *testing.T) {
	provider := &mockAIProvider{
		response: `{"advice":"Add an index on column id","score":85}`,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	advice, err := svc.SQLAdvice(ctx, "SELECT * FROM users WHERE id = 1", "SeqScan", "users(id)")
	require.NoError(t, err)
	assert.Equal(t, 85, advice.Score)
	assert.Equal(t, "SELECT * FROM users WHERE id = 1", advice.SQLText)
	assert.Equal(t, "SeqScan", advice.Explain)
}

func TestAIServiceSQLAdviceProviderError(t *testing.T) {
	provider := &mockAIProvider{
		err: assert.AnError,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	_, err := svc.SQLAdvice(ctx, "SELECT 1", "", "")
	require.Error(t, err)
}

func TestAIServiceListDiagnoses(t *testing.T) {
	provider := &mockAIProvider{
		response: `{"status":"ok","summary":"good","score":100}`,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	_, _ = svc.Diagnosis(ctx, "inst-1")
	_, _ = svc.Diagnosis(ctx, "inst-1")
	_, _ = svc.Diagnosis(ctx, "inst-2")

	records, err := svc.ListDiagnoses(ctx, "inst-1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestAIServiceGetDiagnosis(t *testing.T) {
	provider := &mockAIProvider{
		response: `{"status":"ok","summary":"good","score":100}`,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	created, _ := svc.Diagnosis(ctx, "inst-1")

	got, err := svc.GetDiagnosis(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestAIServiceListSQLAdvice(t *testing.T) {
	provider := &mockAIProvider{
		response: `{"advice":"use index","score":80}`,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	_, _ = svc.SQLAdvice(ctx, "SELECT 1", "", "")
	_, _ = svc.SQLAdvice(ctx, "SELECT 2", "", "")

	adviceList, err := svc.ListSQLAdvice(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, adviceList, 2)
}

func TestAIServiceGetSQLAdvice(t *testing.T) {
	provider := &mockAIProvider{
		response: `{"advice":"use index","score":80}`,
	}
	svc := newTestAIService(t, provider)
	ctx := context.Background()

	created, _ := svc.SQLAdvice(ctx, "SELECT 1", "", "")

	got, err := svc.GetSQLAdvice(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestAIServiceProviderReturnsProvider(t *testing.T) {
	provider := &mockAIProvider{}
	svc := newTestAIService(t, provider)

	assert.Equal(t, provider, svc.Provider())
}
