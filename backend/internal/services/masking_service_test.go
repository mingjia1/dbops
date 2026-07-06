package services

import (
	"context"
	"strings"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMaskingService(t *testing.T) (*MaskingService, *repositories.MaskingRepository) {
	db := newTestDB(t)
	repo := repositories.NewMaskingRepository(db)
	return NewMaskingService(repo), repo
}

func TestMaskingServiceCreate(t *testing.T) {
	svc, _ := newTestMaskingService(t)
	ctx := context.Background()

	rule := &models.MaskingRule{
		ID:          "rule-1",
		Name:        "mask-email",
		FieldPath:   "user.email",
		Algorithm:   models.MaskingMask,
		Enabled:     true,
	}
	err := svc.Create(ctx, rule)
	require.NoError(t, err)

	got, err := svc.GetByID(ctx, "rule-1")
	require.NoError(t, err)
	assert.Equal(t, "mask-email", got.Name)
}

func TestMaskingServiceList(t *testing.T) {
	svc, _ := newTestMaskingService(t)
	ctx := context.Background()

	_ = svc.Create(ctx, &models.MaskingRule{ID: "r1", Name: "rule1", Algorithm: models.MaskingMD5, Enabled: true})
	_ = svc.Create(ctx, &models.MaskingRule{ID: "r2", Name: "rule2", Algorithm: models.MaskingMask, Enabled: false})

	rules, err := svc.List(ctx)
	require.NoError(t, err)
	assert.Len(t, rules, 2)
}

func TestMaskingServiceUpdate(t *testing.T) {
	svc, _ := newTestMaskingService(t)
	ctx := context.Background()

	_ = svc.Create(ctx, &models.MaskingRule{ID: "r1", Name: "old", Algorithm: models.MaskingMD5})

	err := svc.Update(ctx, &models.MaskingRule{ID: "r1", Name: "new", Algorithm: models.MaskingReplace})
	require.NoError(t, err)

	got, err := svc.GetByID(ctx, "r1")
	require.NoError(t, err)
	assert.Equal(t, "new", got.Name)
}

func TestMaskingServiceDelete(t *testing.T) {
	svc, _ := newTestMaskingService(t)
	ctx := context.Background()

	_ = svc.Create(ctx, &models.MaskingRule{ID: "r1", Name: "rule1"})
	err := svc.Delete(ctx, "r1")
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, "r1")
	require.Error(t, err)
}

func TestMaskingServiceGetEnabledRules(t *testing.T) {
	svc, _ := newTestMaskingService(t)
	ctx := context.Background()

	_ = svc.Create(ctx, &models.MaskingRule{ID: "r1", Name: "enabled", Enabled: true})
	_ = svc.Create(ctx, &models.MaskingRule{ID: "r2", Name: "disabled", Enabled: false})
	_ = svc.Create(ctx, &models.MaskingRule{ID: "r3", Name: "enabled2", Enabled: true})

	rules, err := svc.GetEnabledRules(ctx)
	require.NoError(t, err)
	assert.Len(t, rules, 2)
	for _, r := range rules {
		assert.True(t, r.Enabled)
	}
}

func TestMaskFieldMD5(t *testing.T) {
	svc, _ := newTestMaskingService(t)

	rule := &models.MaskingRule{Algorithm: models.MaskingMD5}
	assert.Equal(t, "***", svc.MaskField("hello", rule))
	assert.Equal(t, "", svc.MaskField("", rule))
}

func TestMaskFieldMask(t *testing.T) {
	svc, _ := newTestMaskingService(t)

	rule := &models.MaskingRule{Algorithm: models.MaskingMask}

	// Short value (<=4): mask all
	result := svc.MaskField("ab", rule)
	assert.Equal(t, "**", result)

	// Normal value: keep first 2 and last 2
	result = svc.MaskField("hello-world", rule)
	assert.True(t, strings.HasPrefix(result, "he"))
	assert.True(t, strings.HasSuffix(result, "ld"))
	assert.Contains(t, result, "***")
}

func TestMaskFieldReplace(t *testing.T) {
	svc, _ := newTestMaskingService(t)

	rule := &models.MaskingRule{
		Algorithm:   models.MaskingReplace,
		Replacement: "XXX",
	}

	result := svc.MaskField("abc123def", rule)
	assert.Equal(t, "XXX", result)
}

func TestMaskFieldReplaceInvalidRegex(t *testing.T) {
	svc, _ := newTestMaskingService(t)

	rule := &models.MaskingRule{
		Algorithm:   models.MaskingReplace,
		Pattern:     `[invalid`,
		Replacement: "XXX",
	}

	result := svc.MaskField("hello", rule)
	assert.Equal(t, "XXX", result)
}

func TestMaskFieldReplaceNoReplacement(t *testing.T) {
	svc, _ := newTestMaskingService(t)

	rule := &models.MaskingRule{
		Algorithm:   models.MaskingReplace,
		Pattern:     `secret`,
		Replacement: "",
	}

	result := svc.MaskField("my secret data", rule)
	assert.Equal(t, "***", result)
}

func TestMaskFieldDefault(t *testing.T) {
	svc, _ := newTestMaskingService(t)

	rule := &models.MaskingRule{Algorithm: "unknown"}
	assert.Equal(t, "***", svc.MaskField("hello", rule))
}
