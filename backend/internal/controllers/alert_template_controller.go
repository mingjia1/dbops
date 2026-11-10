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
		utils.InternalServerErrorResponse(ctx, "获取告警模板列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, templates)
}

func (c *AlertTemplateController) Create(ctx *gin.Context) {
	var tpl models.AlertTemplate
	if err := ctx.ShouldBindJSON(&tpl); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	if err := c.service.CreateAlertTemplate(ctx.Request.Context(), &tpl); err != nil {
		utils.InternalServerErrorResponse(ctx, "创建告警模板失败", err)
		return
	}
	utils.SuccessResponse(ctx, tpl)
}

func (c *AlertTemplateController) Get(ctx *gin.Context) {
	id := ctx.Param("id")
	tpl, err := c.service.GetAlertTemplate(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "告警模板不存在")
		return
	}
	utils.SuccessResponse(ctx, tpl)
}

func (c *AlertTemplateController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteAlertTemplate(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除告警模板失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}
