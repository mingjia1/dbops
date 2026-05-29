package controllers

import (
	"strconv"
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
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
	resourceType := ctx.Query("resource_type")
	resourceID := ctx.Query("resource_id")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	var auditLogs []models.AuditLog
	if userID != "" {
		auditLogs, err = c.service.ListAuditLogsByUser(ctx.Request.Context(), userID, limit, offset)
	} else if resourceType != "" && resourceID != "" {
		auditLogs, err = c.service.ListAuditLogsByResource(ctx.Request.Context(), resourceType, resourceID, limit, offset)
	} else {
		auditLogs, err = c.service.ListAuditLogs(ctx.Request.Context(), limit, offset)
	}

	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list audit logs", err)
		return
	}

	utils.SuccessResponse(ctx, auditLogs)
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