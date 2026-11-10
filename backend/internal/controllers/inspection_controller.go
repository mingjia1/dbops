package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type InspectionController struct {
	service *services.AlertService
}

func NewInspectionController(service *services.AlertService) *InspectionController {
	return &InspectionController{service: service}
}

// --- Templates ---

func (c *InspectionController) ListTemplates(ctx *gin.Context) {
	category := ctx.Query("category")
	list, err := c.service.ListInspectionTemplates(ctx.Request.Context(), category)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取巡检模板列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *InspectionController) CreateTemplate(ctx *gin.Context) {
	var t models.InspectionTemplate
	if err := ctx.ShouldBindJSON(&t); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	if err := c.service.CreateInspectionTemplate(ctx.Request.Context(), &t); err != nil {
		utils.InternalServerErrorResponse(ctx, "创建巡检模板失败", err)
		return
	}
	utils.SuccessResponse(ctx, t)
}

func (c *InspectionController) GetTemplate(ctx *gin.Context) {
	id := ctx.Param("id")
	t, err := c.service.GetInspectionTemplate(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "巡检模板不存在")
		return
	}
	utils.SuccessResponse(ctx, t)
}

func (c *InspectionController) UpdateTemplate(ctx *gin.Context) {
	id := ctx.Param("id")
	var t models.InspectionTemplate
	if err := ctx.ShouldBindJSON(&t); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	t.ID = id
	if err := c.service.UpdateInspectionTemplate(ctx.Request.Context(), &t); err != nil {
		utils.InternalServerErrorResponse(ctx, "更新巡检模板失败", err)
		return
	}
	utils.SuccessResponse(ctx, t)
}

func (c *InspectionController) DeleteTemplate(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteInspectionTemplate(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除巡检模板失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}

// --- Reports ---

type GenerateReportRequest struct {
	TemplateID string `json:"template_id" binding:"required"`
	InstanceID string `json:"instance_id" binding:"required"`
}

func (c *InspectionController) GenerateReport(ctx *gin.Context) {
	var req GenerateReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	report, err := c.service.GenerateInspectionReport(ctx.Request.Context(), req.TemplateID, req.InstanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "生成巡检报告失败", err)
		return
	}
	utils.SuccessResponse(ctx, report)
}

func (c *InspectionController) ListReports(ctx *gin.Context) {
	templateID := ctx.Query("template_id")
	limit := 20
	offset := 0
	if l := ctx.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if o := ctx.Query("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}
	list, err := c.service.ListInspectionReports(ctx.Request.Context(), templateID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取巡检报告列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *InspectionController) GetReport(ctx *gin.Context) {
	id := ctx.Param("id")
	report, err := c.service.GetInspectionReport(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "巡检报告不存在")
		return
	}
	utils.SuccessResponse(ctx, report)
}

func (c *InspectionController) DeleteReport(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteInspectionReport(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除巡检报告失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}
