package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type SwitchController struct {
	service *services.SwitchService
}

func NewSwitchController(service *services.SwitchService) *SwitchController {
	return &SwitchController{service: service}
}

func (c *SwitchController) SingleToMHA(ctx *gin.Context) {
	var req services.SwitchClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	result, err := c.service.SingleToMHA(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Switch failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *SwitchController) SingleToMGR(ctx *gin.Context) {
	var req services.SwitchClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	result, err := c.service.SingleToMGR(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Switch failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *SwitchController) SingleToPXC(ctx *gin.Context) {
	var req services.SwitchClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	result, err := c.service.SingleToPXC(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Switch failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *SwitchController) SwitchRoleWithinCluster(ctx *gin.Context) {
	var req services.RoleSwitchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	result, err := c.service.SwitchRoleWithinCluster(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Role switch failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *SwitchController) ListRoleSwitchHistory(ctx *gin.Context) {
	clusterID := ctx.Param("cluster_id")
	if clusterID == "" {
		utils.BadRequestResponse(ctx, "cluster_id is required")
		return
	}
	limit := 50
	if v := ctx.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	history, err := c.service.ListRoleSwitchHistory(ctx.Request.Context(), clusterID, limit)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list role switch history", err)
		return
	}
	utils.SuccessResponse(ctx, history)
}
