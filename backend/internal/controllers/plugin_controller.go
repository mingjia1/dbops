package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/plugins"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type PluginController struct {
	registry *plugins.Registry
	repo     *repositories.PluginRepository
}

func NewPluginController(registry *plugins.Registry, repo *repositories.PluginRepository) *PluginController {
	return &PluginController{registry: registry, repo: repo}
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

// Register 接入一个外部 HTTP 动态插件 (免重编译, 配置持久化到 plugin_registry).
func (c *PluginController) Register(ctx *gin.Context) {
	var req plugins.HTTPPluginConfig
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体格式有误")
		return
	}
	if err := req.Validate(); err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}

	p := plugins.NewHTTPPlugin(req)
	if err := c.registry.RegisterDynamic(p); err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}
	raw, err := json.Marshal(p.Config())
	if err != nil {
		_ = c.registry.Unregister(req.Name)
		utils.ErrorResponse(ctx, http.StatusInternalServerError, "插件配置序列化失败", err)
		return
	}
	if err := c.repo.Upsert(ctx.Request.Context(), repositories.PluginRecord{
		Name:    req.Name,
		Type:    p.Info().Type,
		Version: p.Info().Version,
		Config:  string(raw),
	}); err != nil {
		// 内存与 DB 保持一致: 持久化失败就撤销本次接入.
		_ = c.registry.Unregister(req.Name)
		utils.ErrorResponse(ctx, http.StatusInternalServerError, "插件已接入但持久化失败", err)
		return
	}
	utils.SuccessResponse(ctx, p.Info())
}

// Unregister 取消注册动态插件. 内置插件受保护, 不可注销.
func (c *PluginController) Unregister(ctx *gin.Context) {
	name := ctx.Param("name")
	info, ok := c.registry.GetInfo(name)
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "plugin not found"})
		return
	}
	if info.Source != "dynamic" {
		utils.ErrorResponse(ctx, http.StatusForbidden, "内置插件不可取消注册", nil)
		return
	}
	// 先删 DB 再删内存, 避免 DB 删除失败导致重启后插件复活.
	if err := c.repo.Delete(ctx.Request.Context(), name); err != nil {
		utils.ErrorResponse(ctx, http.StatusInternalServerError, "取消注册失败", err)
		return
	}
	if err := c.registry.Unregister(name); err != nil {
		utils.ErrorResponse(ctx, http.StatusInternalServerError, "取消注册失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"name": name, "unregistered": true})
}

// Probe 在正式接入前验证外部插件服务的连通性 (POST <endpoint>/ping).
func (c *PluginController) Probe(ctx *gin.Context) {
	var req struct {
		Endpoint  string `json:"endpoint"`
		Token     string `json:"token"`
		TimeoutMS int    `json:"timeout_ms"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体格式有误")
		return
	}
	cfg := plugins.HTTPPluginConfig{
		Name:      "probe",
		Endpoint:  req.Endpoint,
		Token:     req.Token,
		TimeoutMS: req.TimeoutMS,
	}
	if cfg.TimeoutMS == 0 {
		cfg.TimeoutMS = 5000
	}
	if err := cfg.Validate(); err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}
	if err := plugins.NewHTTPPlugin(cfg).Ping(ctx.Request.Context()); err != nil {
		utils.BadRequestResponse(ctx, "连接测试失败: "+err.Error())
		return
	}
	utils.SuccessResponse(ctx, gin.H{"reachable": true})
}
