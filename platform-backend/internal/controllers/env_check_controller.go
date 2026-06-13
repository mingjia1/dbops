package controllers

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type EnvironmentCheckController struct {
	service *services.EnvironmentCheckService
}

func NewEnvironmentCheckController(service *services.EnvironmentCheckService) *EnvironmentCheckController {
	return &EnvironmentCheckController{service: service}
}

func (c *EnvironmentCheckController) Execute(ctx *gin.Context) {
	var req services.EnvironmentCheckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.Execute(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute environment check", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *EnvironmentCheckController) GetByID(ctx *gin.Context) {
	checkID := ctx.Param("id")
	
	result, err := c.service.GetByID(ctx.Request.Context(), checkID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Check result not found")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *EnvironmentCheckController) Export(ctx *gin.Context) {
	checkID := ctx.Param("id")
	format := ctx.DefaultQuery("format", "json")
	
	result, err := c.service.GetByID(ctx.Request.Context(), checkID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Check result not found")
		return
	}

	if format == "pdf" {
		ctx.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "PDF export feature will be implemented",
			"data":    result,
		})
		return
	}

	utils.SuccessResponse(ctx, result)
}