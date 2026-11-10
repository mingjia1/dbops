package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type SettingsController struct {
	repo *repositories.PlatformSettingsRepository
}

func NewSettingsController(repo *repositories.PlatformSettingsRepository) *SettingsController {
	return &SettingsController{repo: repo}
}

func (c *SettingsController) GetAll(ctx *gin.Context) {
	settings, err := c.repo.List(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取设置失败", err)
		return
	}
	utils.SuccessResponse(ctx, settings)
}

func (c *SettingsController) Get(ctx *gin.Context) {
	key := ctx.Param("key")
	value, err := c.repo.Get(ctx.Request.Context(), key)
	if err != nil {
		utils.NotFoundResponse(ctx, "设置项不存在")
		return
	}
	utils.SuccessResponse(ctx, gin.H{"key": key, "value": value})
}

func (c *SettingsController) Set(ctx *gin.Context) {
	key := ctx.Param("key")
	var body struct {
		Value string `json:"value"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		utils.BadRequestResponse(ctx, "请求无效")
		return
	}
	if err := c.repo.Set(ctx.Request.Context(), key, body.Value); err != nil {
		utils.InternalServerErrorResponse(ctx, "保存设置失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"key": key, "value": body.Value})
}

func (c *SettingsController) Delete(ctx *gin.Context) {
	key := ctx.Param("key")
	if err := c.repo.Delete(ctx.Request.Context(), key); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除设置项失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"key": key, "deleted": true})
}
