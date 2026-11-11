package controllers

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

// knownSettingKeys: 可通过 API 写入的设置键白名单.
// 注意: audit_chain_hmac_key 由后端内部生成并固化, 不在白名单内 ——
// 若允许前端改写, 换钥后历史审计链将全部无法验证.
var knownSettingKeys = map[string]bool{
	"password_policy":     true,
	"metrics_thresholds":  true,
	"platform_params":     true,
	"relay_config":        true,
	"credential_password": true,
	"auth.default_role":   true,
	"auth.ldap_enabled":   true,
	"auth.local_enabled":  true,
	"auth.oauth2_enabled": true,
}

// validateSettingValue: 对已知 JSON 结构的 key 做服务端 schema 校验, 拦截畸形值.
func validateSettingValue(key, value string) error {
	switch key {
	case "password_policy":
		var p struct {
			MinLength       int `json:"min_length"`
			MaxAgeDays      int `json:"max_age_days"`
			HistoryCount    int `json:"history_count"`
			RequireUppercase bool `json:"require_uppercase"`
			RequireLowercase bool `json:"require_lowercase"`
			RequireDigit     bool `json:"require_digit"`
			RequireSpecial   bool `json:"require_special"`
		}
		if err := json.Unmarshal([]byte(value), &p); err != nil {
			return fmt.Errorf("password_policy 不是合法 JSON: %v", err)
		}
		if p.MinLength != 0 && (p.MinLength < 4 || p.MinLength > 64) {
			return fmt.Errorf("min_length 必须在 4~64 之间")
		}
		if p.MaxAgeDays < 0 || p.MaxAgeDays > 3650 {
			return fmt.Errorf("max_age_days 必须在 0~3650 之间")
		}
		if p.HistoryCount < 0 || p.HistoryCount > 24 {
			return fmt.Errorf("history_count 必须在 0~24 之间")
		}
	case "metrics_thresholds", "platform_params", "relay_config":
		if !json.Valid([]byte(value)) {
			return fmt.Errorf("%s 必须是合法 JSON", key)
		}
	}
	return nil
}

type SettingsController struct {
	repo     *repositories.PlatformSettingsRepository
	auditSvc *services.AuditService
}

func NewSettingsController(repo *repositories.PlatformSettingsRepository) *SettingsController {
	return &SettingsController{repo: repo}
}

func (c *SettingsController) SetAuditService(a *services.AuditService) {
	c.auditSvc = a
}

func (c *SettingsController) auditSettingChange(ctx *gin.Context, action, key, value, result, errMsg string) {
	if c.auditSvc == nil {
		return
	}
	details := "key=" + key
	if action == "update" && len(value) > 120 {
		value = value[:120] + "...(truncated)"
	}
	details += " value=" + value
	_, _ = c.auditSvc.CreateAuditLog(ctx.Request.Context(), services.CreateAuditLogRequest{
		UserID:       userIDFromGinCtx(ctx),
		Operation:    "settings_" + action,
		ResourceType: "setting",
		ResourceID:   key,
		Action:       action,
		Details:      details,
		Result:       result,
		ErrorMsg:     errMsg,
		IPAddress:    ctx.ClientIP(),
		UserAgent:    ctx.Request.UserAgent(),
	})
}

// userIDFromGinCtx: 从 gin 上下文取当前用户 ID (与 auth middleware 写入键一致).
func userIDFromGinCtx(ctx *gin.Context) string {
	if v, ok := ctx.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (c *SettingsController) GetAll(ctx *gin.Context) {
	settings, err := c.repo.List(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取设置失败", err)
		return
	}
	utils.SuccessResponse(ctx, settings)
}

func (c *SettingsController) Get(ctx *gin.Context) {
	key := ctx.Param("key")
	value, err := c.repo.Get(ctx.Request.Context(), key)
	if err != nil {
		utils.NotFoundResponse(ctx, "设置项不存在")
		return
	}
	utils.SuccessResponse(ctx, gin.H{"key": key, "value": value})
}

func (c *SettingsController) Set(ctx *gin.Context) {
	key := ctx.Param("key")
	if !knownSettingKeys[key] {
		utils.BadRequestResponse(ctx, "未知设置项: "+key+" (不在允许写入的白名单内)")
		return
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		utils.BadRequestResponse(ctx, "请求无效")
		return
	}
	if err := validateSettingValue(key, body.Value); err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}
	if err := c.repo.Set(ctx.Request.Context(), key, body.Value); err != nil {
		c.auditSettingChange(ctx, "update", key, body.Value, "failed", err.Error())
		utils.InternalServerErrorResponse(ctx, "保存设置失败", err)
		return
	}
	c.auditSettingChange(ctx, "update", key, body.Value, "success", "")
	utils.SuccessResponse(ctx, gin.H{"key": key, "value": body.Value})
}

func (c *SettingsController) Delete(ctx *gin.Context) {
	key := ctx.Param("key")
	if !knownSettingKeys[key] {
		utils.BadRequestResponse(ctx, "未知设置项: "+key+" (不在允许写入的白名单内)")
		return
	}
	if err := c.repo.Delete(ctx.Request.Context(), key); err != nil {
		c.auditSettingChange(ctx, "delete", key, "", "failed", err.Error())
		utils.InternalServerErrorResponse(ctx, "删除设置项失败", err)
		return
	}
	c.auditSettingChange(ctx, "delete", key, "", "success", "")
	utils.SuccessResponse(ctx, gin.H{"key": key, "deleted": true})
}
