package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestParameterTemplateService(t *testing.T) *ParameterTemplateService {
	db := newTestDB(t)
	return NewParameterTemplateService(repositories.NewParameterTemplateRepository(db))
}

func TestParameterTemplateServiceCreate(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	tpl, err := svc.Create(ctx, CreateParameterTemplateRequest{
		Name:        "Test Template",
		Description: "A test template",
		Category:    "general",
		CreatedBy:   "admin",
		Parameters: []ParameterInput{
			{ParameterName: "innodb_buffer_pool_size", Value: "1G", DataType: "size"},
			{ParameterName: "max_connections", Value: "200", DataType: "int"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Test Template", tpl.Name)
	assert.Equal(t, "general", tpl.Category)
	assert.Equal(t, "admin", tpl.CreatedBy)
}

func TestParameterTemplateServiceGetByID(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateParameterTemplateRequest{
		Name: "Test",
		Parameters: []ParameterInput{
			{ParameterName: "max_connections", Value: "200", DataType: "int"},
		},
	})
	require.NoError(t, err)

	got, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test", got.Name)
	assert.Len(t, got.Parameters, 1)
}

func TestParameterTemplateServiceList(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	_, _ = svc.Create(ctx, CreateParameterTemplateRequest{Name: "T1"})
	_, _ = svc.Create(ctx, CreateParameterTemplateRequest{Name: "T2"})

	tpls, err := svc.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, tpls, 2)
}

func TestParameterTemplateServiceUpdate(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateParameterTemplateRequest{Name: "Old"})
	require.NoError(t, err)

	updated, err := svc.Update(ctx, created.ID, UpdateParameterTemplateRequest{
		Name:        "New",
		Description: "Updated",
	})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
	assert.Equal(t, "Updated", updated.Description)
}

func TestParameterTemplateServiceUpdateWithParameters(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateParameterTemplateRequest{
		Name: "Test",
		Parameters: []ParameterInput{
			{ParameterName: "old_param", Value: "1"},
		},
	})
	require.NoError(t, err)

	newParams := []ParameterInput{
		{ParameterName: "new_param", Value: "2", DataType: "int"},
	}
	updated, err := svc.Update(ctx, created.ID, UpdateParameterTemplateRequest{
		Parameters: &newParams,
	})
	require.NoError(t, err)
	assert.Len(t, updated.Parameters, 1)
	assert.Equal(t, "new_param", updated.Parameters[0].ParameterName)
}

func TestParameterTemplateServiceDelete(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateParameterTemplateRequest{Name: "Test"})
	require.NoError(t, err)

	err = svc.Delete(ctx, created.ID)
	require.NoError(t, err)
}

func TestParameterTemplateServiceGetParameters(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateParameterTemplateRequest{
		Name: "Test",
		Parameters: []ParameterInput{
			{ParameterName: "p1", Value: "v1"},
			{ParameterName: "p2", Value: "v2"},
		},
	})
	require.NoError(t, err)

	params, err := svc.GetParameters(ctx, created.ID, nil)
	require.NoError(t, err)
	assert.Len(t, params, 2)
}

func TestParameterTemplateServiceValidateParametersValid(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	result, err := svc.ValidateParameters(ctx, ValidateParametersRequest{
		Parameters: []ParameterValidationInput{
			{ParameterName: "max_connections", Value: "200", DataType: "int", IsMandatory: true},
			{ParameterName: "read_only", Value: "on", DataType: "bool"},
		},
	})
	require.NoError(t, err)
	assert.True(t, result.IsValid)
	assert.Empty(t, result.Errors)
}

func TestParameterTemplateServiceValidateParametersMissingMandatory(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	result, err := svc.ValidateParameters(ctx, ValidateParametersRequest{
		Parameters: []ParameterValidationInput{
			{ParameterName: "max_connections", Value: "", DataType: "int", IsMandatory: true},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsValid)
	assert.NotEmpty(t, result.Errors)
}

func TestParameterTemplateServiceValidateParametersInvalidInt(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	result, err := svc.ValidateParameters(ctx, ValidateParametersRequest{
		Parameters: []ParameterValidationInput{
			{ParameterName: "max_connections", Value: "abc", DataType: "int"},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsValid)
}

func TestParameterTemplateServiceValidateParametersInvalidBool(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	result, err := svc.ValidateParameters(ctx, ValidateParametersRequest{
		Parameters: []ParameterValidationInput{
			{ParameterName: "read_only", Value: "maybe", DataType: "bool"},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsValid)
}

func TestParameterTemplateServiceValidateParametersRangeViolation(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	result, err := svc.ValidateParameters(ctx, ValidateParametersRequest{
		Parameters: []ParameterValidationInput{
			{ParameterName: "max_connections", Value: "100000", DataType: "int", MinValue: "1", MaxValue: "10000"},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsValid)
}

func TestParameterTemplateServiceValidateParametersIllegalValues(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	result, err := svc.ValidateParameters(ctx, ValidateParametersRequest{
		Parameters: []ParameterValidationInput{
			{ParameterName: "log_error", Value: "file:/etc/passwd", DataType: "string"},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsValid)
}

func TestParameterTemplateServiceRecommendParameters(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	result, err := svc.RecommendParameters(ctx, RecommendParametersRequest{
		CPUCores:    8,
		MemoryGB:    32,
		DiskGB:      500,
		WorkloadType: "oltp",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Recommendations)
	assert.NotEmpty(t, result.Summary)
	assert.Equal(t, "8 cores, 32GB RAM, 500GB disk, oltp workload", result.HardwareProfile)
}

func TestParameterTemplateServiceRecommendParametersOLAP(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	result, err := svc.RecommendParameters(ctx, RecommendParametersRequest{
		CPUCores:    16,
		MemoryGB:    64,
		DiskGB:      2000,
		WorkloadType: "olap",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Recommendations)
}

func TestParameterTemplateServiceListPresets(t *testing.T) {
	svc := newTestParameterTemplateService(t)
	ctx := context.Background()

	presets, err := svc.ListPresets(ctx)
	require.NoError(t, err)
	assert.NotNil(t, presets)
}
