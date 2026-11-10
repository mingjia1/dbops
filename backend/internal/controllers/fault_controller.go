package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type FaultController struct {
	service *services.FaultService
}

func NewFaultController(service *services.FaultService) *FaultController {
	return &FaultController{service: service}
}

// --- Templates ---

func (c *FaultController) ListTemplates(ctx *gin.Context) {
	category := ctx.Query("category")
	list, err := c.service.ListTemplates(ctx.Request.Context(), category)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取故障模板列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *FaultController) CreateTemplate(ctx *gin.Context) {
	var ft models.FaultTemplate
	if err := ctx.ShouldBindJSON(&ft); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	if err := c.service.CreateTemplate(ctx.Request.Context(), &ft); err != nil {
		utils.InternalServerErrorResponse(ctx, "创建故障模板失败", err)
		return
	}
	utils.SuccessResponse(ctx, ft)
}

func (c *FaultController) GetTemplate(ctx *gin.Context) {
	id := ctx.Param("id")
	ft, err := c.service.GetTemplate(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "故障模板不存在")
		return
	}
	utils.SuccessResponse(ctx, ft)
}

func (c *FaultController) DeleteTemplate(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteTemplate(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除故障模板失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}

// --- Execution ---

func (c *FaultController) Execute(ctx *gin.Context) {
	var req services.ExecuteFaultRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	exec, err := c.service.ExecuteFault(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "执行故障注入失败", err)
		return
	}
	utils.SuccessResponse(ctx, exec)
}

func (c *FaultController) Rollback(ctx *gin.Context) {
	id := ctx.Param("id")
	exec, err := c.service.RollbackFault(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "回滚故障注入失败", err)
		return
	}
	utils.SuccessResponse(ctx, exec)
}

func (c *FaultController) ListExecutions(ctx *gin.Context) {
	drillID := ctx.Query("drill_id")
	list, err := c.service.ListExecutions(ctx.Request.Context(), drillID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取故障演练执行列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *FaultController) GetExecution(ctx *gin.Context) {
	id := ctx.Param("id")
	exec, err := c.service.GetExecution(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "故障演练执行记录不存在")
		return
	}
	utils.SuccessResponse(ctx, exec)
}
