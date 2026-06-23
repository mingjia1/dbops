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

	var list []plugins.Plugin
	if typeFilter != "" {
		list = c.registry.ListByType(plugins.PluginType(typeFilter))
	} else {
		list = c.registry.List()
	}

	result := make([]pluginInfo, 0, len(list))
	for _, p := range list {
		result = append(result, pluginInfo{
			Name:    p.Name(),
			Type:    string(p.Type()),
			Version: p.Version(),
		})
	}

	utils.SuccessResponse(ctx, result)
}

func (c *PluginController) Get(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		utils.BadRequestResponse(ctx, "plugin name is required")
		return
	}

	p, err := c.registry.Get(name)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "plugin not found"})
		return
	}

	utils.SuccessResponse(ctx, pluginInfo{
		Name:    p.Name(),
		Type:    string(p.Type()),
		Version: p.Version(),
	})
}
