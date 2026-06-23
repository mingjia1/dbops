package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type AlertNotificationRepository struct {
	db *Database
}

func NewAlertNotificationRepository(db *Database) *AlertNotificationRepository {
	return &AlertNotificationRepository{db: db}
}

func (r *AlertNotificationRepository) CreateAlertNotification(ctx context.Context, notification *models.NotificationChannel) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}

	notification.ID = uuid.New().String()

	query := `
		INSERT INTO notification_channels (id, name, channel_type, channel_config, template, is_active, priority, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		notification.ID, notification.Name, notification.ChannelType, notification.ChannelConfig,
		notification.Template, notification.IsActive, notification.Priority, notification.Description,
		notification.CreatedAt, notification.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create alert notification: %w", err)
	}

	return nil
}

func (r *AlertNotificationRepository) UpdateAlertNotification(ctx context.Context, notification *models.NotificationChannel) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}

	query := `
		UPDATE notification_channels
		SET name = ?, channel_type = ?, channel_config = ?, template = ?, is_active = ?, priority = ?, description = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		notification.Name, notification.ChannelType, notification.ChannelConfig,
		notification.Template, notification.IsActive, notification.Priority, notification.Description,
		notification.UpdatedAt, notification.ID)

	if err != nil {
		return fmt.Errorf("failed to update alert notification: %w", err)
	}

	return nil
}

func (r *AlertNotificationRepository) DeleteAlertNotification(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}

	query := `DELETE FROM notification_channels WHERE id = ?`
	_, err := r.db.Pool.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete alert notification: %w", err)
	}
	return nil
}

func (r *AlertNotificationRepository) GetAlertNotificationByID(ctx context.Context, id string) (*models.NotificationChannel, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available in standalone mode")
	}

	query := `
		SELECT id, name, channel_type, channel_config, template, is_active, priority, description, created_at, updated_at
		FROM notification_channels WHERE id = ?
	`

	notification := &models.NotificationChannel{}
	var name, chType, chCfg, tmpl, desc sql.NullString
	var updatedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&notification.ID, &name, &chType, &chCfg,
		&tmpl, &notification.IsActive, &notification.Priority, &desc,
		&notification.CreatedAt, &updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("alert notification not found")
		}
		return nil, fmt.Errorf("failed to get alert notification: %w", err)
	}
	if name.Valid {
		notification.Name = name.String
	}
	if chType.Valid {
		notification.ChannelType = chType.String
	}
	if chCfg.Valid {
		notification.ChannelConfig = chCfg.String
	}
	if tmpl.Valid {
		notification.Template = tmpl.String
	}
	if desc.Valid {
		notification.Description = desc.String
	}
	if updatedAt.Valid {
		notification.UpdatedAt = updatedAt.Time
	}

	return notification, nil
}

func (r *AlertNotificationRepository) ListAlertNotifications(ctx context.Context, limit, offset int) ([]models.NotificationChannel, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.NotificationChannel{}, nil
	}

	query := `
		SELECT id, name, channel_type, channel_config, template, is_active, priority, description, created_at, updated_at
		FROM notification_channels ORDER BY priority DESC, created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list alert notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.NotificationChannel
	for rows.Next() {
		var notification models.NotificationChannel
		var name, chType, chCfg, tmpl, desc sql.NullString
		var updatedAt sql.NullTime
		if err := rows.Scan(&notification.ID, &name, &chType, &chCfg,
			&tmpl, &notification.IsActive, &notification.Priority, &desc,
			&notification.CreatedAt, &updatedAt); err != nil {
			return nil, err
		}
		if name.Valid {
			notification.Name = name.String
		}
		if chType.Valid {
			notification.ChannelType = chType.String
		}
		if chCfg.Valid {
			notification.ChannelConfig = chCfg.String
		}
		if tmpl.Valid {
			notification.Template = tmpl.String
		}
		if desc.Valid {
			notification.Description = desc.String
		}
		if updatedAt.Valid {
			notification.UpdatedAt = updatedAt.Time
		}
		notifications = append(notifications, notification)
	}

	return notifications, nil
}

func (r *AlertNotificationRepository) GetActiveNotifications(ctx context.Context) ([]models.NotificationChannel, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.NotificationChannel{}, nil
	}

	query := `
		SELECT id, name, channel_type, channel_config, template, is_active, priority, description, created_at, updated_at
		FROM notification_channels WHERE is_active = 1 ORDER BY priority DESC, created_at DESC
	`

	rows, err := r.db.Pool.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.NotificationChannel
	for rows.Next() {
		var notification models.NotificationChannel
		var name, chType, chCfg, tmpl, desc sql.NullString
		var updatedAt sql.NullTime
		if err := rows.Scan(&notification.ID, &name, &chType, &chCfg,
			&tmpl, &notification.IsActive, &notification.Priority, &desc,
			&notification.CreatedAt, &updatedAt); err != nil {
			return nil, err
		}
		if name.Valid {
			notification.Name = name.String
		}
		if chType.Valid {
			notification.ChannelType = chType.String
		}
		if chCfg.Valid {
			notification.ChannelConfig = chCfg.String
		}
		if tmpl.Valid {
			notification.Template = tmpl.String
		}
		if desc.Valid {
			notification.Description = desc.String
		}
		if updatedAt.Valid {
			notification.UpdatedAt = updatedAt.Time
		}
		notifications = append(notifications, notification)
	}

	return notifications, nil
}
