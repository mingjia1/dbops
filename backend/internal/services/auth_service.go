package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo    *repositories.UserRepository
	roleRepo    *repositories.RoleRepository
	auditSvc    *AuditService
	db          interface{}
	standalone  bool
	jwtSecret   string
	tokenExpiry time.Duration
}

func NewAuthService(userRepo *repositories.UserRepository, jwtSecret string, auditSvc ...*AuditService) *AuthService {
	var db interface{} = userRepo
	var audit *AuditService
	if len(auditSvc) > 0 {
		audit = auditSvc[0]
	}
	tokenExpiry := 24 * time.Hour
	if v := os.Getenv("DBOPS_JWT_EXPIRY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			tokenExpiry = d
		}
	}
	return &AuthService{
		userRepo:    userRepo,
		auditSvc:    audit,
		db:          db,
		standalone:  userRepo == nil,
		jwtSecret:   jwtSecret,
		tokenExpiry: tokenExpiry,
	}
}

func (s *AuthService) IsStandalone() bool {
	return s.standalone
}

func (s *AuthService) SetRoleRepository(roleRepo *repositories.RoleRepository) {
	s.roleRepo = roleRepo
}

type LoginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	IPAddress string `json:"-"`
	UserAgent string `json:"-"`
}

type LoginResponse struct {
	Token     string   `json:"token"`
	ExpiresAt int64    `json:"expires_at"`
	User      UserInfo `json:"user"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

type ResetAllPasswordsRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type UserInfo struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type Claims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	// Standalone 模式：允许任意用户名密码登录
	if s.IsStandalone() {
		log.Printf("[SECURITY] WARNING: Standalone mode active — all authentication bypassed")
		expiresAt := time.Now().Add(s.tokenExpiry)
		claims := &Claims{
			UserID:      "standalone-user",
			Username:    req.Username,
			Role:        "admin",
			Permissions: []string{"*"},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(s.jwtSecret))
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}

		return &LoginResponse{
			Token:     tokenString,
			ExpiresAt: expiresAt.Unix(),
			User: UserInfo{
				ID:          "standalone-user",
				Username:    req.Username,
				Role:        "admin",
				Roles:       []string{"admin"},
				Permissions: []string{"*"},
			},
		}, nil
	}

	// 正常模式：从数据库验证用户
	if s.userRepo == nil {
		return nil, errors.New("user repository not available")
	}

	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if user == nil {
		return nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	if user.Status != "active" {
		return nil, errors.New("user account is not active")
	}

	permissions, roles := s.permissionsForUser(ctx, user)
	expiresAt := time.Now().Add(s.tokenExpiry)
	claims := &Claims{
		UserID:      user.ID,
		Username:    user.Username,
		Role:        user.Role,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	now := time.Now()
	_ = s.userRepo.UpdateLoginMetadata(ctx, user.ID, req.IPAddress, now)
	auditCtx := context.WithValue(ctx, "user_id", user.ID)
	s.auditAuth(auditCtx, "login", "login", "user", user.ID, "success", "", "source="+user.Source, req.IPAddress, req.UserAgent)

	return &LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt.Unix(),
		User: UserInfo{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Email:       user.Email,
			Role:        user.Role,
			Roles:       roles,
			Permissions: permissions,
		},
	}, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if !s.IsStandalone() && s.userRepo != nil {
			user, err := s.userRepo.GetByID(context.Background(), claims.UserID)
			if err != nil {
				return nil, err
			}
			if user == nil || user.Status != "active" {
				return nil, errors.New("user account is not active")
			}
			permissions, _ := s.permissionsForUser(context.Background(), user)
			claims.Permissions = permissions
			claims.Role = user.Role
			claims.Username = user.Username
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func (s *AuthService) HasPermission(role, permission string) bool {
	rolePermissions := map[string][]string{
		"admin":     {"*"},
		"dba":       {"instance:*", "deploy:*", "upgrade:*", "backup:*", "restore:*", "monitor:view"},
		"operator":  {"instance:view", "deploy:execute", "backup:execute", "restore:execute", "monitor:view"},
		"developer": {"instance:view_own", "backup:apply", "monitor:view_own"},
		"auditor":   {"instance:view", "monitor:view", "audit:view"},
	}

	permissions := rolePermissions[role]
	prefix := ""
	if idx := strings.Index(permission, ":"); idx > 0 {
		prefix = permission[:idx]
	}
	for _, p := range permissions {
		if p == "*" || p == permission {
			return true
		}
		if prefix != "" && strings.HasSuffix(p, ":*") && strings.HasPrefix(p, prefix+":") {
			return true
		}
	}
	return false
}

func (s *AuthService) HasAnyPermission(permissions []string, permission string) bool {
	if permission == "" {
		return true
	}
	for _, p := range permissions {
		if permissionMatches(p, permission) {
			return true
		}
	}
	return false
}

// Register P0-2: 真实实现 - bcrypt 哈希 + 写库, 不再是 no-op.
func (s *AuthService) ChangePassword(ctx context.Context, userID string, req ChangePasswordRequest) error {
	if s.IsStandalone() || s.userRepo == nil {
		return errors.New("password change is not available in standalone mode")
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return err
	}
	s.auditAuth(ctx, "change_password", "change_password", "user", userID, "success", "", "user changed own password", "", "")
	return nil
}

func (s *AuthService) ResetAllPasswords(ctx context.Context, req ResetAllPasswordsRequest) (int64, error) {
	if s.IsStandalone() || s.userRepo == nil {
		return 0, errors.New("password reset is not available in standalone mode")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("hash password: %w", err)
	}
	updated, err := s.userRepo.UpdateAllPasswords(ctx, string(hash))
	if err != nil {
		return 0, err
	}
	s.auditAuth(ctx, "reset_all_passwords", "reset_password", "user", "all", "success", "", fmt.Sprintf("updated_count=%d", updated), "", "")
	return updated, nil
}

func (s *AuthService) auditAuth(ctx context.Context, operation, action, resourceType, resourceID, result, errorMsg, details, ip, userAgent string) {
	if s.auditSvc == nil {
		return
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    operation,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Details:      details,
		Result:       result,
		ErrorMsg:     errorMsg,
		IPAddress:    ip,
		UserAgent:    userAgent,
	})
}

func (s *AuthService) Register(ctx context.Context, username, password, email, role string) error {
	if s.IsStandalone() || s.userRepo == nil {
		return errors.New("registration is not available in standalone mode")
	}
	if existing, _ := s.userRepo.GetByUsername(ctx, username); existing != nil {
		return errors.New("username already exists")
	}
	if err := utils.ValidatePasswordComplexity(password); err != nil {
		return fmt.Errorf("password does not meet complexity requirements: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if role == "" {
		role = "operator"
	}
	if err := s.userRepo.Create(ctx, &models.User{
		Username:  username,
		Password:  string(hash),
		Email:     email,
		Role:      role,
		Status:    "active",
		Source:    "local",
		CreatedAt: time.Now(),
	}); err != nil {
		return err
	}
	if s.roleRepo != nil {
		if user, err := s.userRepo.GetByUsername(ctx, username); err == nil && user != nil {
			_ = s.roleRepo.SetUserRolesByName(ctx, user.ID, []string{role})
		}
	}
	return nil
}

// SeedAdminIfEmpty 首次启动 + users 表为空时, 创建一个 admin 账号并返回明文密码.
// 密码仅返回一次, 调用方必须落到日志里提示用户首次登录后修改.
func (s *AuthService) SeedAdminIfEmpty(ctx context.Context) (created bool, username, plainPassword string, err error) {
	if s.IsStandalone() || s.userRepo == nil {
		return false, "", "", nil
	}
	users, err := s.userRepo.List(ctx, 1, 0)
	if err != nil {
		return false, "", "", err
	}
	if len(users) > 0 {
		return false, "", "", nil
	}
	username = "admin"
	plainPassword, err = GenerateSecurePassword(24)
	if err != nil {
		return false, "", "", fmt.Errorf("generate admin bootstrap password: %w", err)
	}
	hash, hashErr := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if hashErr != nil {
		return false, "", "", hashErr
	}
	if createErr := s.userRepo.Create(ctx, &models.User{
		Username:  username,
		Password:  string(hash),
		Email:     "admin@localhost",
		Role:      "admin",
		Status:    "active",
		Source:    "local",
		CreatedAt: time.Now(),
	}); createErr != nil {
		return false, "", "", createErr
	}
	if s.roleRepo != nil {
		if user, err := s.userRepo.GetByUsername(ctx, username); err == nil && user != nil {
			_ = s.roleRepo.SetUserRolesByName(ctx, user.ID, []string{"admin"})
		}
	}
	return true, username, plainPassword, nil
}

func (s *AuthService) CurrentUser(ctx context.Context, userID string) (*UserInfo, error) {
	if s.IsStandalone() {
		return &UserInfo{ID: "standalone-user", Username: "standalone", Role: "admin", Roles: []string{"admin"}, Permissions: []string{"*"}}, nil
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != "active" {
		return nil, errors.New("user account is not active")
	}
	permissions, roles := s.permissionsForUser(ctx, user)
	return &UserInfo{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Role:        user.Role,
		Roles:       roles,
		Permissions: permissions,
	}, nil
}

func (s *AuthService) permissionsForUser(ctx context.Context, user *models.User) ([]string, []string) {
	if user == nil {
		return nil, nil
	}
	if s.roleRepo != nil {
		roles, err := s.roleRepo.ListByUserID(ctx, user.ID)
		if err == nil && len(roles) > 0 {
			return mergeRolePermissions(roles), roleNames(roles)
		}
		if role, err := s.roleRepo.GetByName(ctx, user.Role); err == nil && role != nil {
			return role.Permissions, []string{role.Name}
		}
	}
	rolePermissions := map[string][]string{
		"admin":     {"*"},
		"dba":       {"instance:*", "deploy:*", "upgrade:*", "backup:*", "restore:*", "monitor:view"},
		"operator":  {"instance:view", "deploy:execute", "backup:execute", "restore:execute", "monitor:view"},
		"developer": {"instance:view_own", "backup:apply", "monitor:view_own"},
		"auditor":   {"instance:view", "monitor:view", "audit:view"},
		"viewer":    {"instance:view", "host:read", "monitor:view"},
	}
	return rolePermissions[user.Role], []string{user.Role}
}

func mergeRolePermissions(roles []models.Role) []string {
	seen := map[string]bool{}
	var out []string
	for _, role := range roles {
		for _, permission := range role.Permissions {
			if !seen[permission] {
				out = append(out, permission)
				seen[permission] = true
			}
		}
	}
	return out
}

func roleNames(roles []models.Role) []string {
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		out = append(out, role.Name)
	}
	return out
}

func permissionMatches(granted, required string) bool {
	if granted == "*" || granted == required {
		return true
	}
	if required == "admin" {
		return granted == "*"
	}
	if strings.HasSuffix(granted, ":*") {
		prefix := strings.TrimSuffix(granted, "*")
		return strings.HasPrefix(required, prefix)
	}
	return false
}
