package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAuditService(t *testing.T) (*AuditService, *repositories.AuditLogRepository) {
	db := newTestDB(t)
	auditRepo := repositories.NewAuditLogRepository(db)
	svc := NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))
	svc.SetHMACSecret("test-hmac-secret")
	return svc, auditRepo
}

func TestAuditServiceCreateAuditLog(t *testing.T) {
	svc, repo := newTestAuditService(t)
	ctx := context.Background()

	log, err := svc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       "user-001",
		Operation:    "login",
		ResourceType: "auth",
		Action:       "login",
		Result:       "success",
	})
	require.NoError(t, err)
	assert.Equal(t, "user-001", log.UserID)
	assert.Equal(t, "login", log.Operation)
	assert.NotEmpty(t, log.Hash)

	fetched, err := repo.GetByID(ctx, log.ID)
	require.NoError(t, err)
	assert.Equal(t, log.Hash, fetched.Hash)
}

func TestAuditServiceCreateMultipleLogsChain(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	log1, err := svc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID: "user-001", Operation: "op1", Result: "success",
	})
	require.NoError(t, err)
	assert.Empty(t, log1.PrevHash)

	log2, err := svc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID: "user-001", Operation: "op2", Result: "success",
	})
	require.NoError(t, err)
	assert.Equal(t, log1.Hash, log2.PrevHash)

	log3, err := svc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID: "user-001", Operation: "op3", Result: "success",
	})
	require.NoError(t, err)
	assert.Equal(t, log2.Hash, log3.PrevHash)
}

func TestAuditServiceVerifyChain(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	log1, err := svc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID: "user-001", Operation: "op1", Result: "success",
	})
	require.NoError(t, err)
	_, err = svc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID: "user-001", Operation: "op2", Result: "success",
	})
	require.NoError(t, err)

	valid, msg, err := svc.VerifyChain(ctx, log1.ID)
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Contains(t, msg, "intact")
}

func TestAuditServiceVerifyChainSingleLog(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	log1, err := svc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID: "user-001", Operation: "op1", Result: "success",
	})
	require.NoError(t, err)

	valid, msg, err := svc.VerifyChain(ctx, log1.ID)
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Contains(t, msg, "intact")
}

func TestAuditServiceVerifyChainInvalidID(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	_, _, err := svc.VerifyChain(ctx, "missing")
	require.Error(t, err)
}

func TestAuditServiceListAuditLogs(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID: "user-001", Operation: "op", Result: "success",
		})
	}

	logs, err := svc.ListAuditLogs(ctx, 3, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

func TestAuditServiceListAuditLogsByUser(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{UserID: "user-001", Operation: "op1", Result: "success"})
	_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{UserID: "user-002", Operation: "op2", Result: "success"})
	_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{UserID: "user-001", Operation: "op3", Result: "success"})

	logs, err := svc.ListAuditLogsByUser(ctx, "user-001", 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
	for _, l := range logs {
		assert.Equal(t, "user-001", l.UserID)
	}
}

func TestAuditServiceListAuditLogsByResource(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{UserID: "user-001", ResourceType: "instance", ResourceID: "inst-1", Operation: "op", Result: "success"})
	_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{UserID: "user-001", ResourceType: "instance", ResourceID: "inst-2", Operation: "op", Result: "success"})
	_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{UserID: "user-001", ResourceType: "host", ResourceID: "host-1", Operation: "op", Result: "success"})

	logs, err := svc.ListAuditLogsByResource(ctx, "instance", "inst-1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

func TestAuditServiceGetAuditLogByID(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	log, err := svc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID: "user-001", Operation: "op1", Result: "success",
	})
	require.NoError(t, err)

	fetched, err := svc.GetAuditLogByID(ctx, log.ID)
	require.NoError(t, err)
	assert.Equal(t, log.ID, fetched.ID)
}

func TestAuditServiceListAuditLogsFiltered(t *testing.T) {
	svc, _ := newTestAuditService(t)
	ctx := context.Background()

	_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{UserID: "user-001", Operation: "login", ResourceType: "auth", Result: "success"})
	_, _ = svc.CreateAuditLog(ctx, CreateAuditLogRequest{UserID: "user-002", Operation: "deploy", ResourceType: "instance", Result: "success"})

	result, err := svc.ListAuditLogsFiltered(ctx, repositories.AuditLogFilter{
		UserID: "user-001",
	}, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Len(t, result.Logs, 1)
}
