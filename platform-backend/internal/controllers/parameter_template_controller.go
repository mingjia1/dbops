package controllers

import (
	"strconv"
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type ParameterTemplateController struct {
	service *services.ParameterTemplateService
}

func NewParameterTemplateController(service *services.ParameterTemplateService) *ParameterTemplateController {
	return &ParameterTemplateController{service: service}
}

func (c *ParameterTemplateController) Create(ctx *gin.Context) {
	var req services.CreateParameterTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	template, err := c.service.Create(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create parameter template", err)
		return
	}

	utils.SuccessResponse(ctx, template)
}

func (c *ParameterTemplateController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")

	template, err := c.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Parameter template not found")
		return
	}

	utils.SuccessResponse(ctx, template)
}

func (c *ParameterTemplateController) List(ctx *gin.Context) {
	limitStr := ctx.DefaultQuery("limit", "20")
	offsetStr := ctx.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	templates, err := c.service.List(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list parameter templates", err)
		return
	}

	utils.SuccessResponse(ctx, templates)
}

func (c *ParameterTemplateController) ListPresets(ctx *gin.Context) {
	templates, err := c.service.ListPresets(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list preset templates", err)
		return
	}

	utils.SuccessResponse(ctx, templates)
}

func (c *ParameterTemplateController) Update(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.UpdateParameterTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	template, err := c.service.Update(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to update parameter template", err)
		return
	}

	utils.SuccessResponse(ctx, template)
}

func (c *ParameterTemplateController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to delete parameter template", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Parameter template deleted successfully"})
}

func (c *ParameterTemplateController) GetParameters(ctx *gin.Context) {
	id := ctx.Param("id")
	versionID := ctx.Query("version_id")

	var versionPtr *string
	if versionID != "" {
		versionPtr = &versionID
	}

	parameters, err := c.service.GetParameters(ctx.Request.Context(), id, versionPtr)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to get parameters", err)
		return
	}

	utils.SuccessResponse(ctx, parameters)
}

func (c *ParameterTemplateController) Validate(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.ValidateParametersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	req.TemplateID = id

	result, err := c.service.ValidateParameters(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to validate parameters", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *ParameterTemplateController) Recommend(ctx *gin.Context) {
	var req services.RecommendParametersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	if req.CPUCores == 0 {
		req.CPUCores = 4
	}
	if req.MemoryGB == 0 {
		req.MemoryGB = 16
	}
	if req.DiskGB == 0 {
		req.DiskGB = 100
	}
	if req.WorkloadType == "" {
		req.WorkloadType = "mixed"
	}

	result, err := c.service.RecommendParameters(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to generate recommendations", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *ParameterTemplateController) Apply(ctx *gin.Context) {
	var req services.ApplyParameterTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.Apply(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to apply parameter template", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}
