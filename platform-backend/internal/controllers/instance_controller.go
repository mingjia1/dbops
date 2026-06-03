package controllers

import (
	"strconv"
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
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
	limitStr := ctx.DefaultQuery("limit", "20")
	offsetStr := ctx.DefaultQuery("offset", "0")
	hostID := ctx.Query("host_id")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	var instances []models.Instance
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