package repositories

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type ParameterTemplateRepository struct {
	db *Database
}

func NewParameterTemplateRepository(db *Database) *ParameterTemplateRepository {
	return &ParameterTemplateRepository{db: db}
}

func (r *ParameterTemplateRepository) Create(ctx context.Context, template *models.ParameterTemplate) error {
	template.ID = uuid.New().String()

	query := `
		INSERT INTO parameter_templates (id, name, description, category, is_preset, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		template.ID, template.Name, template.Description, template.Category,
		template.IsPreset, template.CreatedBy, template.CreatedAt, template.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create parameter template: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) GetByID(ctx context.Context, id string) (*models.ParameterTemplate, error) {
	query := `
		SELECT id, name, description, category, is_preset, created_by, created_at, updated_at
		FROM parameter_templates WHERE id = $1
	`

	template := &models.ParameterTemplate{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&template.ID, &template.Name, &template.Description, &template.Category,
		&template.IsPreset, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("parameter template not found")
		}
		return nil, fmt.Errorf("failed to get parameter template: %w", err)
	}

	return template, nil
}

func (r *ParameterTemplateRepository) GetByName(ctx context.Context, name string) (*models.ParameterTemplate, error) {
	query := `
		SELECT id, name, description, category, is_preset, created_by, created_at, updated_at
		FROM parameter_templates WHERE name = $1
	`

	template := &models.ParameterTemplate{}
	err := r.db.Pool.QueryRow(ctx, query, name).Scan(
		&template.ID, &template.Name, &template.Description, &template.Category,
		&template.IsPreset, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("parameter template not found")
		}
		return nil, fmt.Errorf("failed to get parameter template: %w", err)
	}

	return template, nil
}

func (r *ParameterTemplateRepository) List(ctx context.Context, limit, offset int) ([]models.ParameterTemplate, error) {
	query := `
		SELECT id, name, description, category, is_preset, created_by, created_at, updated_at
		FROM parameter_templates ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list parameter templates: %w", err)
	}
	defer rows.Close()

	var templates []models.ParameterTemplate
	for rows.Next() {
		var template models.ParameterTemplate
		if err := rows.Scan(
			&template.ID, &template.Name, &template.Description, &template.Category,
			&template.IsPreset, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, nil
}

func (r *ParameterTemplateRepository) ListPresetTemplates(ctx context.Context) ([]models.ParameterTemplate, error) {
	query := `
		SELECT id, name, description, category, is_preset, created_by, created_at, updated_at
		FROM parameter_templates WHERE is_preset = true ORDER BY category, name
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list preset templates: %w", err)
	}
	defer rows.Close()

	var templates []models.ParameterTemplate
	for rows.Next() {
		var template models.ParameterTemplate
		if err := rows.Scan(
			&template.ID, &template.Name, &template.Description, &template.Category,
			&template.IsPreset, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, nil
}

func (r *ParameterTemplateRepository) Update(ctx context.Context, template *models.ParameterTemplate) error {
	query := `
		UPDATE parameter_templates
		SET name = $2, description = $3, category = $4, updated_at = $5
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query,
		template.ID, template.Name, template.Description, template.Category, template.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update parameter template: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM parameter_templates WHERE id = $1 AND is_preset = false`
	_, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete parameter template: %w", err)
	}
	return nil
}

func (r *ParameterTemplateRepository) CreateVersion(ctx context.Context, version *models.ParameterTemplateVersion) error {
	version.ID = uuid.New().String()

	query := `
		INSERT INTO parameter_template_versions (id, template_id, version, description, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		version.ID, version.TemplateID, version.Version, version.Description, version.IsActive, version.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create template version: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) GetVersionByID(ctx context.Context, id string) (*models.ParameterTemplateVersion, error) {
	query := `
		SELECT id, template_id, version, description, is_active, created_at
		FROM parameter_template_versions WHERE id = $1
	`

	version := &models.ParameterTemplateVersion{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&version.ID, &version.TemplateID, &version.Version, &version.Description, &version.IsActive, &version.CreatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("template version not found")
		}
		return nil, fmt.Errorf("failed to get template version: %w", err)
	}

	return version, nil
}

func (r *ParameterTemplateRepository) ListVersions(ctx context.Context, templateID string) ([]models.ParameterTemplateVersion, error) {
	query := `
		SELECT id, template_id, version, description, is_active, created_at
		FROM parameter_template_versions WHERE template_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to list template versions: %w", err)
	}
	defer rows.Close()

	var versions []models.ParameterTemplateVersion
	for rows.Next() {
		var version models.ParameterTemplateVersion
		if err := rows.Scan(
			&version.ID, &version.TemplateID, &version.Version, &version.Description,
			&version.IsActive, &version.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, nil
}

func (r *ParameterTemplateRepository) CreateParameter(ctx context.Context, param *models.ParameterTemplateParameter) error {
	param.ID = uuid.New().String()

	query := `
		INSERT INTO parameter_template_parameters (
			id, template_id, version_id, parameter_name, value, data_type,
			min_value, max_value, unit, description, is_dynamic, is_mandatory, category
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		param.ID, param.TemplateID, param.VersionID, param.ParameterName, param.Value, param.DataType,
		param.MinValue, param.MaxValue, param.Unit, param.Description, param.IsDynamic, param.IsMandatory, param.Category)

	if err != nil {
		return fmt.Errorf("failed to create template parameter: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) GetParameterByID(ctx context.Context, id string) (*models.ParameterTemplateParameter, error) {
	query := `
		SELECT id, template_id, version_id, parameter_name, value, data_type,
			min_value, max_value, unit, description, is_dynamic, is_mandatory, category
		FROM parameter_template_parameters WHERE id = $1
	`

	param := &models.ParameterTemplateParameter{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&param.ID, &param.TemplateID, &param.VersionID, &param.ParameterName, &param.Value, &param.DataType,
		&param.MinValue, &param.MaxValue, &param.Unit, &param.Description, &param.IsDynamic, &param.IsMandatory, &param.Category)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("template parameter not found")
		}
		return nil, fmt.Errorf("failed to get template parameter: %w", err)
	}

	return param, nil
}

func (r *ParameterTemplateRepository) ListParameters(ctx context.Context, templateID string, versionID *string) ([]models.ParameterTemplateParameter, error) {
	var query string
	var args []interface{}

	if versionID != nil && *versionID != "" {
		query = `
			SELECT id, template_id, version_id, parameter_name, value, data_type,
				min_value, max_value, unit, description, is_dynamic, is_mandatory, category
			FROM parameter_template_parameters WHERE template_id = $1 AND version_id = $2
			ORDER BY category, parameter_name
		`
		args = []interface{}{templateID, *versionID}
	} else {
		query = `
			SELECT id, template_id, version_id, parameter_name, value, data_type,
				min_value, max_value, unit, description, is_dynamic, is_mandatory, category
			FROM parameter_template_parameters WHERE template_id = $1
			ORDER BY category, parameter_name
		`
		args = []interface{}{templateID}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list template parameters: %w", err)
	}
	defer rows.Close()

	var params []models.ParameterTemplateParameter
	for rows.Next() {
		var param models.ParameterTemplateParameter
		if err := rows.Scan(
			&param.ID, &param.TemplateID, &param.VersionID, &param.ParameterName, &param.Value, &param.DataType,
			&param.MinValue, &param.MaxValue, &param.Unit, &param.Description, &param.IsDynamic, &param.IsMandatory, &param.Category); err != nil {
			return nil, err
		}
		params = append(params, param)
	}

	return params, nil
}

func (r *ParameterTemplateRepository) UpdateParameter(ctx context.Context, param *models.ParameterTemplateParameter) error {
	query := `
		UPDATE parameter_template_parameters
		SET parameter_name = $2, value = $3, data_type = $4, min_value = $5, max_value = $6,
			unit = $7, description = $8, is_dynamic = $9, is_mandatory = $10, category = $11
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query,
		param.ID, param.ParameterName, param.Value, param.DataType, param.MinValue, param.MaxValue,
		param.Unit, param.Description, param.IsDynamic, param.IsMandatory, param.Category)

	if err != nil {
		return fmt.Errorf("failed to update template parameter: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) DeleteParameter(ctx context.Context, id string) error {
	query := `DELETE FROM parameter_template_parameters WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete template parameter: %w", err)
	}
	return nil
}