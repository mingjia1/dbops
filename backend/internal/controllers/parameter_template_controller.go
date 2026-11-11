package controllers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
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
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	template, err := c.service.Create(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建参数模板失败", err)
		return
	}

	utils.SuccessResponse(ctx, template)
}

func (c *ParameterTemplateController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")

	template, err := c.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "参数模板不存在")
		return
	}

	utils.SuccessResponse(ctx, template)
}

func (c *ParameterTemplateController) List(ctx *gin.Context) {
	limit, offset := parsePagination(ctx)

	templates, err := c.service.List(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取参数模板列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, templates)
}

func (c *ParameterTemplateController) ListPresets(ctx *gin.Context) {
	templates, err := c.service.ListPresets(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取预置模板列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, templates)
}

func (c *ParameterTemplateController) Update(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.UpdateParameterTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	template, err := c.service.Update(ctx.Request.Context(), id, req)
	if err != nil {
		if strings.Contains(err.Error(), "cannot modify preset") {
			utils.BadRequestResponse(ctx, "预设模板不可修改")
			return
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "不存在") {
			utils.NotFoundResponse(ctx, "参数模板不存在")
			return
		}
		utils.InternalServerErrorResponse(ctx, "更新参数模板失败", err)
		return
	}

	utils.SuccessResponse(ctx, template)
}

func (c *ParameterTemplateController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		if strings.Contains(err.Error(), "cannot delete preset") {
			utils.BadRequestResponse(ctx, "预设模板不可删除")
			return
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "不存在") {
			utils.NotFoundResponse(ctx, "参数模板不存在")
			return
		}
		utils.InternalServerErrorResponse(ctx, "删除参数模板失败", err)
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
		utils.InternalServerErrorResponse(ctx, "获取参数失败", err)
		return
	}

	utils.SuccessResponse(ctx, parameters)
}

func (c *ParameterTemplateController) Validate(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.ValidateParametersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	req.TemplateID = id

	result, err := c.service.ValidateParameters(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "参数校验失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *ParameterTemplateController) Recommend(ctx *gin.Context) {
	var req services.RecommendParametersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
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
		utils.InternalServerErrorResponse(ctx, "生成升级建议失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *ParameterTemplateController) Apply(ctx *gin.Context) {
	var req services.ApplyParameterTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.Apply(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "应用参数模板失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}
