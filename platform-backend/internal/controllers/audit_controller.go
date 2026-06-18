package controllers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
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
			utils.BadRequestResponse(ctx, "Invalid start_date")
			return
		}
		filter.StartTime = &t
	}
	if endDate != "" {
		t, parseErr := parseAuditTime(endDate, true)
		if parseErr != nil {
			utils.BadRequestResponse(ctx, "Invalid end_date")
			return
		}
		filter.EndTime = &t
	}

	result, err := c.service.ListAuditLogsFiltered(ctx.Request.Context(), filter, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list audit logs", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"logs": result.Logs, "total": result.Total, "limit": limit, "offset": offset})
}

func (c *AuditController) GetAuditLogByID(ctx *gin.Context) {
	id := ctx.Param("id")

	auditLog, err := c.service.GetAuditLogByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Audit log not found")
		return
	}

	utils.SuccessResponse(ctx, auditLog)
}

func (c *AuditController) CreateAuditLog(ctx *gin.Context) {
	var req services.CreateAuditLogRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	auditLog, err := c.service.CreateAuditLog(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create audit log", err)
		return
	}

	utils.SuccessResponse(ctx, auditLog)
}

func (c *AuditController) VerifyChain(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		id = ctx.Query("id")
	}
	if id == "" {
		utils.BadRequestResponse(ctx, "id is required")
		return
	}

	ok, msg, err := c.service.VerifyChain(ctx.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.NotFoundResponse(ctx, err.Error())
			return
		}
		utils.InternalServerErrorResponse(ctx, "Chain verification failed", err)
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
