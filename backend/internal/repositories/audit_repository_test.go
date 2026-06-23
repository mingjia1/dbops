package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAuditLogRepositoryListFiltered(t *testing.T) {
	db := newRepoTestDB()
	repo := NewAuditLogRepository(db)
	ctx := context.Background()

	base := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	rows := []*models.AuditLog{
		{
			ID:           "audit-1",
			UserID:       "alice",
			Operation:    "approve_request",
			Action:       "approve",
			ResourceType: "approval_request",
			ResourceID:   "approval-1",
			Result:       "success",
			CreatedAt:    base,
		},
		{
			ID:           "audit-2",
			UserID:       "bob",
			Operation:    "create_backup",
			Action:       "create",
			ResourceType: "backup",
			ResourceID:   "backup-1",
			Result:       "success",
			CreatedAt:    base.Add(time.Hour),
		},
		{
			ID:           "audit-3",
			UserID:       "alice-ops",
			Operation:    "reject_request",
			Action:       "reject",
			ResourceType: "approval_request",
			ResourceID:   "approval-2",
			Result:       "success",
			CreatedAt:    base.Add(2 * time.Hour),
		},
	}
	for _, row := range rows {
		require.NoError(t, repo.Create(ctx, row))
	}

	got, total, err := repo.ListFiltered(ctx, AuditLogFilter{UserID: "alice", Action: "request", ResourceType: "approval_request"}, 10, 0)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, got, 2)
	require.Equal(t, "audit-3", got[0].ID)
	require.Equal(t, "audit-1", got[1].ID)

	start := base.Add(30 * time.Minute)
	end := base.Add(90 * time.Minute)
	got, total, err = repo.ListFiltered(ctx, AuditLogFilter{StartTime: &start, EndTime: &end}, 10, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, got, 1)
	require.Equal(t, "audit-2", got[0].ID)

	got, total, err = repo.ListFiltered(ctx, AuditLogFilter{ResourceType: "backup", ResourceID: "backup-1"}, 10, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, got, 1)
	require.Equal(t, "audit-2", got[0].ID)
}
