package controllers

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type InstanceController struct {
	service *services.InstanceService
}

func NewInstanceController(service *services.InstanceService) *InstanceController {
	return &InstanceController{service: service}
}

func (c *InstanceController) Create(ctx *gin.Context) {
	var req services.CreateInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	instance, err := c.service.Create(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create instance", err)
		return
	}

	utils.SuccessResponse(ctx, instance)
}

func (c *InstanceController) BatchCreate(ctx *gin.Context) {
	var req services.BatchCreateInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.BatchCreate(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create instances", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *InstanceController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")

	instance, err := c.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Instance not found")
		return
	}

	utils.SuccessResponse(ctx, instance)
}

func (c *InstanceController) List(ctx *gin.Context) {
	limit, offset := parsePagination(ctx)
	hostID := ctx.Query("host_id")

	var instances []models.Instance
	var err error
	if hostID != "" {
		instances, err = c.service.ListByHostID(ctx.Request.Context(), hostID, limit, offset)
	} else {
		instances, err = c.service.List(ctx.Request.Context(), limit, offset)
	}
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list instances", err)
		return
	}

	utils.SuccessResponse(ctx, instances)
}

func (c *InstanceController) Update(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.UpdateInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	instance, err := c.service.Update(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to update instance", err)
		return
	}

	utils.SuccessResponse(ctx, instance)
}

func (c *InstanceController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to delete instance", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Instance deleted successfully"})
}

func (c *InstanceController) DetectVersion(ctx *gin.Context) {
	id := ctx.Param("id")

	version, err := c.service.DetectVersion(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to detect version", err)
		return
	}

	utils.SuccessResponse(ctx, version)
}

func (c *InstanceController) Deploy(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.service.Deploy(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to deploy instance", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *InstanceController) HealthCheck(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.service.HealthCheck(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to check instance health", err)
		return
	}

	if isFailedInstanceTaskStatus(result.Status) {
		ctx.JSON(424, utils.Response{
			Code:      424,
			Message:   "health check failed",
			Data:      result,
			Timestamp: time.Now(),
			TraceID:   ctx.GetString("trace_id"),
		})
		return
	}

	utils.SuccessResponse(ctx, result)
}

func isFailedInstanceTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "unhealthy", "timeout", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func (c *InstanceController) AdminAction(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.InstanceAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.AdminAction(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute instance admin action", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *InstanceController) ForceResetPassword(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.ForceResetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	if err := c.service.ForceResetInstancePassword(ctx.Request.Context(), id, req); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to force reset password", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Password reset successfully"})
}

func (c *InstanceController) UpdateStatus(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.UpdateInstanceStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	instance, err := c.service.UpdateInstanceStatus(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to update instance status", err)
		return
	}

	utils.SuccessResponse(ctx, instance)
}

func (c *InstanceController) GetCredentials(ctx *gin.Context) {
	id := ctx.Param("id")
	adminUser := ctx.GetString("user_id")
	log.Printf("AUDIT: admin=%s accessed credentials for instance=%s", adminUser, id)

	creds, err := c.service.GetInstanceCredentials(ctx.Request.Context(), id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.NotFoundResponse(ctx, "Instance credentials not found")
			return
		}
		utils.InternalServerErrorResponse(ctx, "Failed to retrieve instance credentials", err)
		return
	}

	utils.SuccessResponse(ctx, creds)
}

func (c *InstanceController) BatchUpdatePassword(ctx *gin.Context) {
	var req services.BatchPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.BatchUpdatePassword(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to update instance passwords", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}
