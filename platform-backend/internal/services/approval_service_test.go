package services

import (
	"context"
	"errors"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/require"
)

func newTestApprovalService() (*ApprovalService, *repositories.AuditLogRepository) {
	db := newTestDB()
	approvalRepo := repositories.NewApprovalRequestRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	return NewApprovalService(approvalRepo, auditRepo), auditRepo
}

func TestCreateApprovalRequestWritesAuditLog(t *testing.T) {
	ctx := context.Background()
	service, auditRepo := newTestApprovalService()

	req, err := service.CreateApprovalRequest(ctx, CreateApprovalRequestRequest{
		RequesterID:   "requester-1",
		OperationType: "restart_instance",
		ResourceType:  "instance",
		ResourceID:    "instance-1",
		RequestReason: "maintenance",
		Priority:      2,
		ExpiryHours:   24,
	})

	require.NoError(t, err)
	require.NotEmpty(t, req.ID)

	logs, err := auditRepo.ListByResource(ctx, "approval_request", req.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "create", logs[0].Action)
	require.Equal(t, "requester-1", logs[0].UserID)
}

func TestApproveRequestWritesAuditAndRejectsSecondApproval(t *testing.T) {
	ctx := context.Background()
	service, auditRepo := newTestApprovalService()

	req, err := service.CreateApprovalRequest(ctx, CreateApprovalRequestRequest{
		RequesterID:   "requester-1",
		OperationType: "change_password",
		ResourceType:  "instance",
		ResourceID:    "instance-1",
		Priority:      1,
		ExpiryHours:   24,
	})
	require.NoError(t, err)

	approved, err := service.ApproveRequest(ctx, req.ID, "approver-1", "approved")
	require.NoError(t, err)
	require.Equal(t, "approved", approved.ApprovalStatus)

	_, err = service.ApproveRequest(ctx, req.ID, "approver-1", "again")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrApprovalNotPending))

	logs, err := auditRepo.ListByResource(ctx, "approval_request", req.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	require.Equal(t, "approve", logs[0].Action)
	require.Equal(t, "approver-1", logs[0].UserID)
}
