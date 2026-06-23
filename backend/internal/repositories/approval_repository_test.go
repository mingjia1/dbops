package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/require"
)

func TestApprovalRequestRepositoryUpdateDeleteMissingReturnsError(t *testing.T) {
	db := newRepoTestDB()
	repo := NewApprovalRequestRepository(db)
	ctx := context.Background()

	err := repo.Update(ctx, &models.ApprovalRequest{
		ID:             "missing-approval",
		RequesterID:    "requester-1",
		OperationType:  "restart",
		ResourceType:   "instance",
		ResourceID:     "instance-1",
		ApprovalStatus: "approved",
		ExpiresAt:      time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")

	err = repo.Delete(ctx, "missing-approval")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
