package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type AIController struct {
	service *services.AIService
}

func NewAIController(service *services.AIService) *AIController {
	return &AIController{service: service}
}

type DiagnosisRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
}

func (c *AIController) Diagnosis(ctx *gin.Context) {
	var req DiagnosisRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request body: "+err.Error())
		return
	}
	result, err := c.service.Diagnosis(ctx.Request.Context(), req.InstanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "AI diagnosis failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *AIController) ListDiagnoses(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
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
	list, err := c.service.ListDiagnoses(ctx.Request.Context(), instanceID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list diagnoses", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *AIController) GetDiagnosis(ctx *gin.Context) {
	id := ctx.Param("id")
	d, err := c.service.GetDiagnosis(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Diagnosis not found")
		return
	}
	utils.SuccessResponse(ctx, d)
}

// ListDiagnoses and GetDiagnosis — add to AIService

type SQLAdviceRequest struct {
	SQLText      string `json:"sql_text" binding:"required"`
	ExplainPlan  string `json:"explain"`
	TableSchema  string `json:"schema"`
}

func (c *AIController) SQLAdvice(ctx *gin.Context) {
	var req SQLAdviceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request body: "+err.Error())
		return
	}
	result, err := c.service.SQLAdvice(ctx.Request.Context(), req.SQLText, req.ExplainPlan, req.TableSchema)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "AI SQL advisor failed", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *AIController) ListSQLAdvice(ctx *gin.Context) {
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
	list, err := c.service.ListSQLAdvice(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list SQL advice", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *AIController) GetSQLAdvice(ctx *gin.Context) {
	id := ctx.Param("id")
	a, err := c.service.GetSQLAdvice(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "SQL advice not found")
		return
	}
	utils.SuccessResponse(ctx, a)
}
