package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type HostController struct {
	service *services.HostService
}

func NewHostController(service *services.HostService) *HostController {
	return &HostController{service: service}
}

func (c *HostController) Create(ctx *gin.Context) {
	var req services.CreateHostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	host, err := c.service.Create(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create host", err)
		return
	}

	utils.SuccessResponse(ctx, host)
}

func (c *HostController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")

	host, err := c.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Host not found")
		return
	}

	utils.SuccessResponse(ctx, host)
}

func (c *HostController) List(ctx *gin.Context) {
	limitStr := ctx.DefaultQuery("limit", "20")
	offsetStr := ctx.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	hosts, err := c.service.List(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list hosts", err)
		return
	}

	utils.SuccessResponse(ctx, hosts)
}

func (c *HostController) Update(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.UpdateHostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	host, err := c.service.Update(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to update host", err)
		return
	}

	utils.SuccessResponse(ctx, host)
}

func (c *HostController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to delete host", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Host deleted successfully"})
}

func (c *HostController) TestConnection(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.service.StartTestConnection(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Host not found")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) GetTestResult(ctx *gin.Context) {
	taskID := ctx.Param("task_id")

	result, err := c.service.GetTestResult(taskID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Test task not found")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) ScanInstances(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.ScanInstancesRequest
	_ = ctx.ShouldBindJSON(&req)

	result, err := c.service.StartScanInstances(ctx.Request.Context(), id, req)
	if err != nil {
		utils.NotFoundResponse(ctx, "Host not found")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) GetScanResult(ctx *gin.Context) {
	hostID := ctx.Param("id")
	taskID := ctx.Param("task_id")

	result, err := c.service.GetScanResult(taskID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Scan task not found")
		return
	}
	if result.HostID != hostID {
		utils.NotFoundResponse(ctx, "Scan task does not belong to this host")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) RegisterScannedInstance(ctx *gin.Context) {
	hostID := ctx.Param("id")

	var req services.RegisterScannedInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	instanceID, err := c.service.RegisterScannedInstance(ctx.Request.Context(), hostID, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to register instance", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"instance_id": instanceID, "message": "Instance registered"})
}
