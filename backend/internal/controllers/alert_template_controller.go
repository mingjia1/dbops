package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type AlertTemplateController struct {
	service *services.AlertService
}

func NewAlertTemplateController(service *services.AlertService) *AlertTemplateController {
	return &AlertTemplateController{service: service}
}

func (c *AlertTemplateController) List(ctx *gin.Context) {
	category := ctx.Query("category")
	templates, err := c.service.ListAlertTemplates(ctx.Request.Context(), category)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list alert templates", err)
		return
	}
	utils.SuccessResponse(ctx, templates)
}

func (c *AlertTemplateController) Create(ctx *gin.Context) {
	var tpl models.AlertTemplate
	if err := ctx.ShouldBindJSON(&tpl); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request body: "+err.Error())
		return
	}
	if err := c.service.CreateAlertTemplate(ctx.Request.Context(), &tpl); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create alert template", err)
		return
	}
	utils.SuccessResponse(ctx, tpl)
}

func (c *AlertTemplateController) Get(ctx *gin.Context) {
	id := ctx.Param("id")
	tpl, err := c.service.GetAlertTemplate(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Alert template not found")
		return
	}
	utils.SuccessResponse(ctx, tpl)
}

func (c *AlertTemplateController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteAlertTemplate(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to delete alert template", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}
