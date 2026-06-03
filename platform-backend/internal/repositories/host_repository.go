package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type HostRepository struct {
	db *Database

	mu    sync.RWMutex
	memDB map[string]*models.Host
}

func NewHostRepository(db *Database) *HostRepository {
	return &HostRepository{
		db:    db,
		memDB: make(map[string]*models.Host),
	}
}

func (r *HostRepository) Create(ctx context.Context, host *models.Host) error {
	if r.db == nil || r.db.Pool == nil {
		return r.createInMemory(host)
	}
	return r.createInDB(ctx, host)
}

func (r *HostRepository) createInMemory(host *models.Host) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if host.ID == "" {
		host.ID = uuid.New().String()
	}
	if host.Status == "" {
		host.Status = "unknown"
	}
	now := time.Now()
	host.CreatedAt = now
	host.UpdatedAt = now

	for _, existing := range r.memDB {
		if existing.Name == host.Name {
			return fmt.Errorf("host with name %s already exists", host.Name)
		}
	}

	r.memDB[host.ID] = host
	return nil
}

func (r *HostRepository) createInDB(ctx context.Context, host *models.Host) error {
	if host.ID == "" {
		host.ID = uuid.New().String()
	}
	now := time.Now()
	host.CreatedAt = now
	host.UpdatedAt = now

	query := `
		INSERT INTO hosts (id, name, address, ssh_port, ssh_user, ssh_auth_method, ssh_credential, os_type, description, tags, status, last_check_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Pool.ExecContext(ctx, query,
		host.ID, host.Name, host.Address, host.SSHPort, host.SSHUser, host.SSHAuthMethod,
		nullableString(host.SSHCredential), host.OSType, host.Description, host.Tags, host.Status,
		host.LastCheckAt, host.CreatedAt, host.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}
	return nil
}

func (r *HostRepository) GetByID(ctx context.Context, id string) (*models.Host, error) {
	if r.db == nil || r.db.Pool == nil {
		return r.getByIDInMemory(id)
	}
	return r.getByIDInDB(ctx, id)
}

func (r *HostRepository) getByIDInMemory(id string) (*models.Host, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.memDB[id]
	if !ok {
		return nil, fmt.Errorf("host not found")
	}
	copy := *h
	return &copy, nil
}

func (r *HostRepository) getByIDInDB(ctx context.Context, id string) (*models.Host, error) {
	query := `
		SELECT id, name, address, ssh_port, ssh_user, ssh_auth_method, ssh_credential, os_type, description, tags, status, last_check_at, created_at, updated_at
		FROM hosts WHERE id = ?
	`
	host := &models.Host{}
	var lastCheckAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&host.ID, &host.Name, &host.Address, &host.SSHPort, &host.SSHUser, &host.SSHAuthMethod,
		&host.SSHCredential, &host.OSType, &host.Description, &host.Tags, &host.Status,
		&lastCheckAt, &host.CreatedAt, &host.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("host not found")
		}
		return nil, fmt.Errorf("failed to get host: %w", err)
	}
	if lastCheckAt.Valid {
		t := lastCheckAt.Time
		host.LastCheckAt = &t
	}
	return host, nil
}

func (r *HostRepository) List(ctx context.Context, limit, offset int) ([]models.Host, error) {
	if r.db == nil || r.db.Pool == nil {
		return r.listInMemory(limit, offset)
	}
	return r.listInDB(ctx, limit, offset)
}

func (r *HostRepository) listInMemory(limit, offset int) ([]models.Host, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]models.Host, 0, len(r.memDB))
	for _, h := range r.memDB {
		all = append(all, *h)
	}

	for i := 0; i < len(all)-1; i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].CreatedAt.After(all[i].CreatedAt) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if offset >= len(all) {
		return []models.Host{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (r *HostRepository) listInDB(ctx context.Context, limit, offset int) ([]models.Host, error) {
	query := `
		SELECT id, name, address, ssh_port, ssh_user, ssh_auth_method, ssh_credential, os_type, description, tags, status, last_check_at, created_at, updated_at
		FROM hosts ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %w", err)
	}
	defer rows.Close()

	hosts := make([]models.Host, 0)
	for rows.Next() {
		var h models.Host
		var lastCheckAt sql.NullTime
		if err := rows.Scan(
			&h.ID, &h.Name, &h.Address, &h.SSHPort, &h.SSHUser, &h.SSHAuthMethod,
			&h.SSHCredential, &h.OSType, &h.Description, &h.Tags, &h.Status,
			&lastCheckAt, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		if lastCheckAt.Valid {
			t := lastCheckAt.Time
			h.LastCheckAt = &t
		}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func (r *HostRepository) Update(ctx context.Context, host *models.Host) error {
	if r.db == nil || r.db.Pool == nil {
		return r.updateInMemory(host)
	}
	return r.updateInDB(ctx, host)
}

func (r *HostRepository) updateInMemory(host *models.Host) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.memDB[host.ID]
	if !ok {
		return fmt.Errorf("host not found")
	}

	if host.Name != "" {
		existing.Name = host.Name
	}
	if host.Address != "" {
		existing.Address = host.Address
	}
	if host.SSHPort != 0 {
		existing.SSHPort = host.SSHPort
	}
	if host.SSHUser != "" {
		existing.SSHUser = host.SSHUser
	}
	if host.SSHAuthMethod != "" {
		existing.SSHAuthMethod = host.SSHAuthMethod
	}
	if host.SSHCredential != "" {
		existing.SSHCredential = host.SSHCredential
	}
	if host.OSType != "" {
		existing.OSType = host.OSType
	}
	if host.Description != "" {
		existing.Description = host.Description
	}
	if host.Tags != "" {
		existing.Tags = host.Tags
	}
	existing.UpdatedAt = time.Now()
	return nil
}

func (r *HostRepository) updateInDB(ctx context.Context, host *models.Host) error {
	host.UpdatedAt = time.Now()
	query := `
		UPDATE hosts SET name = ?, address = ?, ssh_port = ?, ssh_user = ?, ssh_auth_method = ?, ssh_credential = ?, os_type = ?, description = ?, tags = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Pool.ExecContext(ctx, query,
		host.Name, host.Address, host.SSHPort, host.SSHUser, host.SSHAuthMethod,
		host.SSHCredential, host.OSType, host.Description, host.Tags, host.UpdatedAt, host.ID)
	if err != nil {
		return fmt.Errorf("failed to update host: %w", err)
	}
	return nil
}

func (r *HostRepository) UpdateStatus(ctx context.Context, id, status string) error {
	now := time.Now()
	if r.db == nil || r.db.Pool == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		if existing, ok := r.memDB[id]; ok {
			existing.Status = status
			existing.LastCheckAt = &now
			existing.UpdatedAt = now
		}
		return nil
	}

	_, err := r.db.Pool.ExecContext(ctx,
		`UPDATE hosts SET status = ?, last_check_at = ?, updated_at = ? WHERE id = ?`,
		status, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to update host status: %w", err)
	}
	return nil
}

func (r *HostRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.memDB, id)
		return nil
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete host: %w", err)
	}
	return nil
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
