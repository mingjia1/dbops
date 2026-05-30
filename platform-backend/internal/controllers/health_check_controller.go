package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type HealthCheckController struct {
	service *services.HealthCheckService
}

func NewHealthCheckController(service *services.HealthCheckService) *HealthCheckController {
	return &HealthCheckController{service: service}
}

func (c *HealthCheckController) ExecuteHealthCheck(ctx *gin.Context) {
	var req services.HealthCheckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.ExecuteHealthCheck(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute health check", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HealthCheckController) DetectFailure(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
	if instanceID == "" {
		utils.BadRequestResponse(ctx, "instance_id is required")
		return
	}

	result, err := c.service.DetectFailure(ctx.Request.Context(), instanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to detect failure", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HealthCheckController) GetFailureState(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
	if instanceID == "" {
		utils.BadRequestResponse(ctx, "instance_id is required")
		return
	}

	state := c.service.GetFailureState(instanceID)
	if state == nil {
		utils.NotFoundResponse(ctx, "Failure state not found")
		return
	}

	utils.SuccessResponse(ctx, state)
}

func (c *HealthCheckController) BatchHealthCheck(ctx *gin.Context) {
	var req struct {
		InstanceIDs []string `json:"instance_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	results, err := c.service.BatchHealthCheck(ctx.Request.Context(), req.InstanceIDs)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute batch health check", err)
		return
	}

	utils.SuccessResponse(ctx, results)
}