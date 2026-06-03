package controllers

import (
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

func (c *SwitchController) MHAToMGR(ctx *gin.Context) {
	var req services.SwitchClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	result, err := c.service.MHAToMGR(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Switch failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *SwitchController) MGRToPXC(ctx *gin.Context) {
	var req services.SwitchClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	result, err := c.service.MGRToPXC(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Switch failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *SwitchController) PXCToMHA(ctx *gin.Context) {
	var req services.SwitchClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	result, err := c.service.PXCToMHA(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Switch failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}
