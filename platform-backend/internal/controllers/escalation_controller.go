package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type EscalationController struct {
	service *services.AlertService
}

func NewEscalationController(service *services.AlertService) *EscalationController {
	return &EscalationController{service: service}
}

func (c *EscalationController) List(ctx *gin.Context) {
	ruleID := ctx.Query("rule_id")
	list, err := c.service.ListEscalations(ctx.Request.Context(), ruleID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list escalations", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *EscalationController) Create(ctx *gin.Context) {
	var e models.AlertEscalation
	if err := ctx.ShouldBindJSON(&e); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request body: "+err.Error())
		return
	}
	if err := c.service.CreateEscalation(ctx.Request.Context(), &e); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create escalation", err)
		return
	}
	utils.SuccessResponse(ctx, e)
}

func (c *EscalationController) Get(ctx *gin.Context) {
	id := ctx.Param("id")
	e, err := c.service.GetEscalation(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Escalation not found")
		return
	}
	utils.SuccessResponse(ctx, e)
}

func (c *EscalationController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteEscalation(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to delete escalation", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}

type SilenceController struct {
	service *services.AlertService
}

func NewSilenceController(service *services.AlertService) *SilenceController {
	return &SilenceController{service: service}
}

func (c *SilenceController) List(ctx *gin.Context) {
	list, err := c.service.ListSilences(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list silences", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *SilenceController) Create(ctx *gin.Context) {
	var s models.AlertSilence
	if err := ctx.ShouldBindJSON(&s); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request body: "+err.Error())
		return
	}
	if err := c.service.CreateSilence(ctx.Request.Context(), &s); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create silence", err)
		return
	}
	utils.SuccessResponse(ctx, s)
}

func (c *SilenceController) Get(ctx *gin.Context) {
	id := ctx.Param("id")
	s, err := c.service.GetSilence(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Silence not found")
		return
	}
	utils.SuccessResponse(ctx, s)
}

func (c *SilenceController) Update(ctx *gin.Context) {
	id := ctx.Param("id")
	var s models.AlertSilence
	if err := ctx.ShouldBindJSON(&s); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request body: "+err.Error())
		return
	}
	s.ID = id
	if err := c.service.UpdateSilence(ctx.Request.Context(), &s); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to update silence", err)
		return
	}
	utils.SuccessResponse(ctx, s)
}

func (c *SilenceController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteSilence(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to delete silence", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}
