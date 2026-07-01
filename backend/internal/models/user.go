package models

import (
	"time"
)

type User struct {
	ID                string     `json:"id"`
	Username          string     `json:"username"`
	Password          string     `json:"-"`
	Email             string     `json:"email"`
	Role              string     `json:"role"`
	Roles             []Role     `json:"roles,omitempty"`
	Permissions       []string   `json:"permissions,omitempty"`
	Status            string     `json:"status"`
	DisplayName       string     `json:"display_name"`
	Phone             string     `json:"phone"`
	Source            string     `json:"source"`
	LastLoginAt       *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP       string     `json:"last_login_ip"`
	PasswordChangedAt *time.Time `json:"password_changed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	IsBuiltin   bool      `json:"is_builtin"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserRole struct {
	UserID string `json:"user_id"`
	RoleID string `json:"role_id"`
}
