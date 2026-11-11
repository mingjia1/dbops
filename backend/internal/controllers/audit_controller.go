package controllers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type AuditController struct {
	service *services.AuditService
}

func NewAuditController(service *services.AuditService) *AuditController {
	return &AuditController{service: service}
}

func (c *AuditController) ListAuditLogs(ctx *gin.Context) {
	limitStr := ctx.DefaultQuery("limit", "20")
	offsetStr := ctx.DefaultQuery("offset", "0")
	userID := ctx.Query("user_id")
	userAlias := ctx.Query("user")
	action := ctx.Query("action")
	resourceType := ctx.Query("resource_type")
	resourceID := ctx.Query("resource_id")
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	if userID == "" {
		userID = userAlias
	}

	filter := repositories.AuditLogFilter{
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
	if startDate != "" {
		t, parseErr := parseAuditTime(startDate, false)
		if parseErr != nil {
			utils.BadRequestResponse(ctx, "start_date 参数无效")
			return
		}
		filter.StartTime = &t
	}
	if endDate != "" {
		t, parseErr := parseAuditTime(endDate, true)
		if parseErr != nil {
			utils.BadRequestResponse(ctx, "end_date 参数无效")
			return
		}
		filter.EndTime = &t
	}

	result, err := c.service.ListAuditLogsFiltered(ctx.Request.Context(), filter, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取审计日志列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"logs": result.Logs, "total": result.Total, "limit": limit, "offset": offset})
}

func (c *AuditController) GetAuditLogByID(ctx *gin.Context) {
	id := ctx.Param("id")

	auditLog, err := c.service.GetAuditLogByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "审计日志不存在")
		return
	}

	utils.SuccessResponse(ctx, auditLog)
}

func (c *AuditController) CreateAuditLog(ctx *gin.Context) {
	var req services.CreateAuditLogRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	auditLog, err := c.service.CreateAuditLog(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建审计日志失败", err)
		return
	}

	utils.SuccessResponse(ctx, auditLog)
}

func (c *AuditController) VerifyChain(ctx *gin.Context) {
	ok, msg, err := c.service.VerifyChain(ctx.Request.Context())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.NotFoundResponse(ctx, err.Error())
			return
		}
		utils.InternalServerErrorResponse(ctx, "审计链校验失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"valid": ok, "message": msg})
}

func parseAuditTime(value string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		return t.Add(24*time.Hour - time.Nanosecond), nil
	}
	return t, nil
}
