package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLicenseService(t *testing.T) (*LicenseService, *repositories.LicenseRepository) {
	db := newTestDB(t)
	licenseRepo := repositories.NewLicenseRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	approvalRepo := repositories.NewApprovalRequestRepository(db)
	auditSvc := NewAuditService(auditRepo, approvalRepo)
	svc := NewLicenseService(licenseRepo, auditSvc, "test-license-secret")
	return svc, licenseRepo
}

func TestLicenseServiceCurrentLicenseEmpty(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	lic, err := svc.CurrentLicense(ctx)
	require.NoError(t, err)
	assert.Nil(t, lic)
}

func TestLicenseServiceCurrentTierDefault(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	tier := svc.CurrentTier(ctx)
	assert.Equal(t, license.TierCE, tier)
}

func TestLicenseServiceHasFeatureDefaultTier(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	assert.True(t, svc.HasFeature(ctx, license.FeatureInstanceManage))
	assert.True(t, svc.HasFeature(ctx, license.FeatureBasicMonitor))
	assert.False(t, svc.HasFeature(ctx, license.FeatureAIAssistant))
}

func TestLicenseServiceUploadLicense(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	// Generate a valid license first
	lic := license.Generate(license.TierEE, "test-user", 365*24*time.Hour, 50, "test-license-secret")
	jsonData, err := json.Marshal(lic)
	require.NoError(t, err)

	saved, err := svc.UploadLicense(ctx, jsonData, "admin")
	require.NoError(t, err)
	assert.Equal(t, license.TierEE, saved.Tier)
	assert.Equal(t, "test-user", saved.IssuedTo)
}

func TestLicenseServiceUploadLicenseDeactivatesOld(t *testing.T) {
	svc, repo := newTestLicenseService(t)
	ctx := context.Background()

	lic1 := license.Generate(license.TierCE, "user1", 365*24*time.Hour, 10, "test-license-secret")
	jsonData1, _ := json.Marshal(lic1)
	_, err := svc.UploadLicense(ctx, jsonData1, "admin")
	require.NoError(t, err)

	lic2 := license.Generate(license.TierEE, "user2", 365*24*time.Hour, 50, "test-license-secret")
	jsonData2, _ := json.Marshal(lic2)
	_, err = svc.UploadLicense(ctx, jsonData2, "admin")
	require.NoError(t, err)

	// Only the latest should be active
	active, err := repo.GetActive(ctx)
	require.NoError(t, err)
	assert.Equal(t, "user2", active.IssuedTo)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	activeCount := 0
	for _, l := range list {
		if l.Active {
			activeCount++
		}
	}
	assert.Equal(t, 1, activeCount)
}

func TestLicenseServiceLicenseInfoNoLicense(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	info, err := svc.LicenseInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ce", info["tier"])
	assert.Equal(t, false, info["active"])
}

func TestLicenseServiceFeatures(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	features := svc.Features(ctx)
	assert.NotEmpty(t, features)
	for _, f := range features {
		assert.True(t, svc.HasFeature(ctx, f))
	}
}

func TestLicenseServiceGenerateLicense(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	lic, err := svc.GenerateLicense(ctx, license.TierUE, "test-org", 365*24*time.Hour, 100)
	require.NoError(t, err)
	assert.Equal(t, license.TierUE, lic.Tier)
	assert.Equal(t, "test-org", lic.IssuedTo)
	assert.Equal(t, 100, lic.MaxNodes)
}

func TestLicenseServiceUploadInvalidJSON(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	_, err := svc.UploadLicense(ctx, []byte("not json"), "admin")
	require.Error(t, err)
}

func TestLicenseServiceUploadInvalidSignature(t *testing.T) {
	svc, _ := newTestLicenseService(t)
	ctx := context.Background()

	// Generate with different secret
	lic := license.Generate(license.TierEE, "user", 365*24*time.Hour, 50, "wrong-secret")
	jsonData, _ := json.Marshal(lic)

	_, err := svc.UploadLicense(ctx, jsonData, "admin")
	require.Error(t, err)
}
