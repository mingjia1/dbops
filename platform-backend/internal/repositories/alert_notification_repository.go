package repositories

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.Pool.Exec(ctx, query,
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
		SET name = $2, channel_type = $3, channel_config = $4, template = $5, is_active = $6, priority = $7, description = $8, updated_at = $9
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query,
		notification.ID, notification.Name, notification.ChannelType, notification.ChannelConfig,
		notification.Template, notification.IsActive, notification.Priority, notification.Description,
		notification.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update alert notification: %w", err)
	}

	return nil
}

func (r *AlertNotificationRepository) DeleteAlertNotification(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}
	
	query := `DELETE FROM notification_channels WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
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
		FROM notification_channels WHERE id = $1
	`

	notification := &models.NotificationChannel{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&notification.ID, &notification.Name, &notification.ChannelType, &notification.ChannelConfig,
		&notification.Template, &notification.IsActive, &notification.Priority, &notification.Description,
		&notification.CreatedAt, &notification.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("alert notification not found")
		}
		return nil, fmt.Errorf("failed to get alert notification: %w", err)
	}

	return notification, nil
}

func (r *AlertNotificationRepository) ListAlertNotifications(ctx context.Context, limit, offset int) ([]models.NotificationChannel, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.NotificationChannel{}, nil
	}
	
	query := `
		SELECT id, name, channel_type, channel_config, template, is_active, priority, description, created_at, updated_at
		FROM notification_channels ORDER BY priority DESC, created_at DESC LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list alert notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.NotificationChannel
	for rows.Next() {
		var notification models.NotificationChannel
		if err := rows.Scan(&notification.ID, &notification.Name, &notification.ChannelType, &notification.ChannelConfig,
			&notification.Template, &notification.IsActive, &notification.Priority, &notification.Description,
			&notification.CreatedAt, &notification.UpdatedAt); err != nil {
			return nil, err
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
		FROM notification_channels WHERE is_active = true ORDER BY priority DESC, created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.NotificationChannel
	for rows.Next() {
		var notification models.NotificationChannel
		if err := rows.Scan(&notification.ID, &notification.Name, &notification.ChannelType, &notification.ChannelConfig,
			&notification.Template, &notification.IsActive, &notification.Priority, &notification.Description,
			&notification.CreatedAt, &notification.UpdatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}

	return notifications, nil
}