package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type DrillController struct {
	service *services.DrillService
}

func NewDrillController(service *services.DrillService) *DrillController {
	return &DrillController{service: service}
}

func (c *DrillController) List(ctx *gin.Context) {
	status := ctx.Query("status")
	list, err := c.service.List(ctx.Request.Context(), status)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取演练列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *DrillController) Create(ctx *gin.Context) {
	var d models.HADrill
	if err := ctx.ShouldBindJSON(&d); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	if err := c.service.Create(ctx.Request.Context(), &d); err != nil {
		utils.InternalServerErrorResponse(ctx, "创建演练失败", err)
		return
	}
	utils.SuccessResponse(ctx, d)
}

func (c *DrillController) Get(ctx *gin.Context) {
	id := ctx.Param("id")
	d, err := c.service.Get(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "演练不存在")
		return
	}
	utils.SuccessResponse(ctx, d)
}

func (c *DrillController) Update(ctx *gin.Context) {
	id := ctx.Param("id")
	var d models.HADrill
	if err := ctx.ShouldBindJSON(&d); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	d.ID = id
	if err := c.service.Update(ctx.Request.Context(), &d); err != nil {
		utils.InternalServerErrorResponse(ctx, "更新演练失败", err)
		return
	}
	utils.SuccessResponse(ctx, d)
}

func (c *DrillController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除演练失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}

func (c *DrillController) Start(ctx *gin.Context) {
	id := ctx.Param("id")
	d, err := c.service.StartDrill(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "启动演练失败", err)
		return
	}
	utils.SuccessResponse(ctx, d)
}

func (c *DrillController) Complete(ctx *gin.Context) {
	id := ctx.Param("id")
	d, report, err := c.service.CompleteDrill(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "完成演练失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"drill": d, "report": report})
}

func (c *DrillController) GetReport(ctx *gin.Context) {
	id := ctx.Param("id")
	report, err := c.service.GetReport(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "演练报告不存在")
		return
	}
	utils.SuccessResponse(ctx, report)
}
