package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/plugins"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type PluginController struct {
	registry *plugins.Registry
}

func NewPluginController(registry *plugins.Registry) *PluginController {
	return &PluginController{registry: registry}
}

type pluginInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

func (c *PluginController) List(ctx *gin.Context) {
	typeFilter := ctx.Query("type")

	all := c.registry.ListAllInfo()
	result := make([]plugins.PluginInfo, 0, len(all))
	for _, info := range all {
		if typeFilter != "" && info.Type != typeFilter {
			continue
		}
		result = append(result, info)
	}

	utils.SuccessResponse(ctx, result)
}

func (c *PluginController) Get(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		utils.BadRequestResponse(ctx, "缺少插件名称")
		return
	}

	info, ok := c.registry.GetInfo(name)
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "plugin not found"})
		return
	}

	utils.SuccessResponse(ctx, info)
}
