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
		utils.InternalServerErrorResponse(ctx, "Failed to list drills", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *DrillController) Create(ctx *gin.Context) {
	var d models.HADrill
	if err := ctx.ShouldBindJSON(&d); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request body: "+err.Error())
		return
	}
	if err := c.service.Create(ctx.Request.Context(), &d); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create drill", err)
		return
	}
	utils.SuccessResponse(ctx, d)
}

func (c *DrillController) Get(ctx *gin.Context) {
	id := ctx.Param("id")
	d, err := c.service.Get(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Drill not found")
		return
	}
	utils.SuccessResponse(ctx, d)
}

func (c *DrillController) Update(ctx *gin.Context) {
	id := ctx.Param("id")
	var d models.HADrill
	if err := ctx.ShouldBindJSON(&d); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request body: "+err.Error())
		return
	}
	d.ID = id
	if err := c.service.Update(ctx.Request.Context(), &d); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to update drill", err)
		return
	}
	utils.SuccessResponse(ctx, d)
}

func (c *DrillController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to delete drill", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}

func (c *DrillController) Start(ctx *gin.Context) {
	id := ctx.Param("id")
	d, err := c.service.StartDrill(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to start drill", err)
		return
	}
	utils.SuccessResponse(ctx, d)
}

func (c *DrillController) Complete(ctx *gin.Context) {
	id := ctx.Param("id")
	d, report, err := c.service.CompleteDrill(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to complete drill", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"drill": d, "report": report})
}

func (c *DrillController) GetReport(ctx *gin.Context) {
	id := ctx.Param("id")
	report, err := c.service.GetReport(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Drill report not found")
		return
	}
	utils.SuccessResponse(ctx, report)
}
