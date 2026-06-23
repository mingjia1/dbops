package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type ParameterTemplateRepository struct {
	db *Database
}

func NewParameterTemplateRepository(db *Database) *ParameterTemplateRepository {
	return &ParameterTemplateRepository{db: db}
}

func (r *ParameterTemplateRepository) Create(ctx context.Context, template *models.ParameterTemplate) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	template.ID = uuid.New().String()
	template.CreatedAt = template.CreatedAt.Round(0)
	template.UpdatedAt = template.UpdatedAt.Round(0)

	query := `
		INSERT INTO parameter_templates (id, name, description, category, is_preset, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		template.ID, template.Name, template.Description, template.Category,
		template.IsPreset, template.CreatedBy, template.CreatedAt, template.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create parameter template: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) GetByID(ctx context.Context, id string) (*models.ParameterTemplate, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, name, description, category, is_preset, created_by, created_at, updated_at
		FROM parameter_templates WHERE id = ?
	`

	template := &models.ParameterTemplate{}
	err := scanParameterTemplate(r.db.Pool.QueryRowContext(ctx, query, id), template)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("parameter template not found")
		}
		return nil, fmt.Errorf("failed to get parameter template: %w", err)
	}

	return template, nil
}

func (r *ParameterTemplateRepository) GetByName(ctx context.Context, name string) (*models.ParameterTemplate, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, name, description, category, is_preset, created_by, created_at, updated_at
		FROM parameter_templates WHERE name = ?
	`

	template := &models.ParameterTemplate{}
	err := scanParameterTemplate(r.db.Pool.QueryRowContext(ctx, query, name), template)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("parameter template not found")
		}
		return nil, fmt.Errorf("failed to get parameter template: %w", err)
	}

	return template, nil
}

func (r *ParameterTemplateRepository) List(ctx context.Context, limit, offset int) ([]models.ParameterTemplate, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.ParameterTemplate{}, nil
	}

	query := `
		SELECT id, name, description, category, is_preset, created_by, created_at, updated_at
		FROM parameter_templates ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list parameter templates: %w", err)
	}
	defer rows.Close()

	templates := make([]models.ParameterTemplate, 0)
	for rows.Next() {
		var template models.ParameterTemplate
		if err := scanParameterTemplate(rows, &template); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, nil
}

func (r *ParameterTemplateRepository) ListPresetTemplates(ctx context.Context) ([]models.ParameterTemplate, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.ParameterTemplate{}, nil
	}

	query := `
		SELECT id, name, description, category, is_preset, created_by, created_at, updated_at
		FROM parameter_templates WHERE is_preset = 1 ORDER BY category, name
	`

	rows, err := r.db.Pool.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list preset templates: %w", err)
	}
	defer rows.Close()

	templates := make([]models.ParameterTemplate, 0)
	for rows.Next() {
		var template models.ParameterTemplate
		if err := scanParameterTemplate(rows, &template); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, nil
}

func (r *ParameterTemplateRepository) Update(ctx context.Context, template *models.ParameterTemplate) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	query := `
		UPDATE parameter_templates
		SET name = ?, description = ?, category = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		template.Name, template.Description, template.Category, template.UpdatedAt,
		template.ID)

	if err != nil {
		return fmt.Errorf("failed to update parameter template: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	query := `DELETE FROM parameter_templates WHERE id = ? AND is_preset = 0`
	_, err := r.db.Pool.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete parameter template: %w", err)
	}
	return nil
}

func (r *ParameterTemplateRepository) CreateVersion(ctx context.Context, version *models.ParameterTemplateVersion) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	version.ID = uuid.New().String()

	query := `
		INSERT INTO parameter_template_versions (id, template_id, version, description, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		version.ID, version.TemplateID, version.Version, version.Description, version.IsActive, version.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create template version: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) GetVersionByID(ctx context.Context, id string) (*models.ParameterTemplateVersion, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, template_id, version, description, is_active, created_at
		FROM parameter_template_versions WHERE id = ?
	`

	version := &models.ParameterTemplateVersion{}
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&version.ID, &version.TemplateID, &version.Version, &version.Description, &version.IsActive, &version.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("template version not found")
		}
		return nil, fmt.Errorf("failed to get template version: %w", err)
	}

	return version, nil
}

func (r *ParameterTemplateRepository) ListVersions(ctx context.Context, templateID string) ([]models.ParameterTemplateVersion, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, template_id, version, description, is_active, created_at
		FROM parameter_template_versions WHERE template_id = ? ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to list template versions: %w", err)
	}
	defer rows.Close()

	versions := make([]models.ParameterTemplateVersion, 0)
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
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	param.ID = uuid.New().String()

	query := `
		INSERT INTO parameter_template_parameters (
			id, template_id, version_id, parameter_name, value, data_type,
			min_value, max_value, unit, description, is_dynamic, is_mandatory, category
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		param.ID, param.TemplateID, nullableText(param.VersionID), param.ParameterName, param.Value, param.DataType,
		param.MinValue, param.MaxValue, param.Unit, param.Description, param.IsDynamic, param.IsMandatory, param.Category)

	if err != nil {
		return fmt.Errorf("failed to create template parameter: %w", err)
	}

	return nil
}

func nullableText(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func (r *ParameterTemplateRepository) GetParameterByID(ctx context.Context, id string) (*models.ParameterTemplateParameter, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, template_id, version_id, parameter_name, value, data_type,
			min_value, max_value, unit, description, is_dynamic, is_mandatory, category
		FROM parameter_template_parameters WHERE id = ?
	`

	param := &models.ParameterTemplateParameter{}
	err := scanParameterTemplateParameter(r.db.Pool.QueryRowContext(ctx, query, id), param)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("template parameter not found")
		}
		return nil, fmt.Errorf("failed to get template parameter: %w", err)
	}

	return param, nil
}

func (r *ParameterTemplateRepository) ListParameters(ctx context.Context, templateID string, versionID *string) ([]models.ParameterTemplateParameter, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var query string
	var args []interface{}

	if versionID != nil && *versionID != "" {
		query = `
			SELECT id, template_id, version_id, parameter_name, value, data_type,
				min_value, max_value, unit, description, is_dynamic, is_mandatory, category
			FROM parameter_template_parameters WHERE template_id = ? AND version_id = ?
			ORDER BY category, parameter_name
		`
		args = []interface{}{templateID, *versionID}
	} else {
		query = `
			SELECT id, template_id, version_id, parameter_name, value, data_type,
				min_value, max_value, unit, description, is_dynamic, is_mandatory, category
			FROM parameter_template_parameters WHERE template_id = ?
			ORDER BY category, parameter_name
		`
		args = []interface{}{templateID}
	}

	rows, err := r.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list template parameters: %w", err)
	}
	defer rows.Close()

	params := make([]models.ParameterTemplateParameter, 0)
	for rows.Next() {
		var param models.ParameterTemplateParameter
		if err := scanParameterTemplateParameter(rows, &param); err != nil {
			return nil, err
		}
		params = append(params, param)
	}

	return params, nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanParameterTemplate(scanner rowScanner, template *models.ParameterTemplate) error {
	var createdAt, updatedAt interface{}
	if err := scanner.Scan(
		&template.ID, &template.Name, &template.Description, &template.Category,
		&template.IsPreset, &template.CreatedBy, &createdAt, &updatedAt); err != nil {
		return err
	}
	template.CreatedAt = parseDBTime(createdAt)
	template.UpdatedAt = parseDBTime(updatedAt)
	return nil
}

func scanParameterTemplateParameter(scanner rowScanner, param *models.ParameterTemplateParameter) error {
	var versionID sql.NullString
	if err := scanner.Scan(
		&param.ID, &param.TemplateID, &versionID, &param.ParameterName, &param.Value, &param.DataType,
		&param.MinValue, &param.MaxValue, &param.Unit, &param.Description, &param.IsDynamic, &param.IsMandatory, &param.Category); err != nil {
		return err
	}
	param.VersionID = versionID.String
	return nil
}

func parseDBTime(value interface{}) time.Time {
	switch v := value.(type) {
	case time.Time:
		return v
	case string:
		return parseTimeString(v)
	case []byte:
		return parseTimeString(string(v))
	default:
		return time.Time{}
	}
}

func parseTimeString(value string) time.Time {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, " m="); idx >= 0 {
		value = value[:idx]
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (r *ParameterTemplateRepository) UpdateParameter(ctx context.Context, param *models.ParameterTemplateParameter) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	query := `
		UPDATE parameter_template_parameters
		SET parameter_name = ?, value = ?, data_type = ?, min_value = ?, max_value = ?,
			unit = ?, description = ?, is_dynamic = ?, is_mandatory = ?, category = ?
		WHERE id = ?
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		param.ParameterName, param.Value, param.DataType, param.MinValue, param.MaxValue,
		param.Unit, param.Description, param.IsDynamic, param.IsMandatory, param.Category,
		param.ID)

	if err != nil {
		return fmt.Errorf("failed to update template parameter: %w", err)
	}

	return nil
}

func (r *ParameterTemplateRepository) DeleteParameter(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	query := `DELETE FROM parameter_template_parameters WHERE id = ?`
	_, err := r.db.Pool.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete template parameter: %w", err)
	}
	return nil
}

func (r *ParameterTemplateRepository) DeleteParametersByTemplate(ctx context.Context, templateID string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	query := `DELETE FROM parameter_template_parameters WHERE template_id = ?`
	_, err := r.db.Pool.ExecContext(ctx, query, templateID)
	if err != nil {
		return fmt.Errorf("failed to delete template parameters: %w", err)
	}
	return nil
}
