package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type HostController struct {
	service *services.HostService
}

func NewHostController(service *services.HostService) *HostController {
	return &HostController{service: service}
}

func (c *HostController) Create(ctx *gin.Context) {
	var req services.CreateHostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	host, err := c.service.Create(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建主机失败", err)
		return
	}

	utils.SuccessResponse(ctx, host)
}

func (c *HostController) BatchCreate(ctx *gin.Context) {
	var req services.BatchCreateHostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.BatchCreate(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "批量创建主机失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")

	host, err := c.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "主机不存在")
		return
	}

	utils.SuccessResponse(ctx, host)
}

func (c *HostController) List(ctx *gin.Context) {
	limit, offset := parsePagination(ctx)

	hosts, err := c.service.List(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取主机列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, hosts)
}

func (c *HostController) Update(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.UpdateHostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	host, err := c.service.Update(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "更新主机失败", err)
		return
	}

	utils.SuccessResponse(ctx, host)
}

func (c *HostController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除主机失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Host deleted successfully"})
}

func (c *HostController) AgentAction(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.HostAgentActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	if services.IsLongRunningAgentAction(req.Action) && !req.Sync {
		result, err := c.service.SubmitAgentAction(ctx.Request.Context(), id, req)
		if err != nil {
					if services.IsAgentTransportError(err) {
			utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Agent 不在线或网络不可达，请检查主机 Agent 状态后重试", err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "提交主机 Agent 操作失败", err)
			return
		}
		utils.SuccessResponse(ctx, result)
		return
	}

	result, err := c.service.AgentAction(ctx.Request.Context(), id, req)
	if err != nil {
				if services.IsAgentTransportError(err) {
			utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Agent 不在线或网络不可达，请检查主机 Agent 状态后重试", err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "执行主机 Agent 操作失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) BatchAgentAction(ctx *gin.Context) {
	var req services.BatchHostAgentActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.BatchAgentAction(ctx.Request.Context(), req)
	if err != nil {
				if services.IsAgentTransportError(err) {
			utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Agent 不在线或网络不可达，请检查主机 Agent 状态后重试", err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "批量执行主机 Agent 操作失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) TestConnection(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.service.StartTestConnection(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "主机不存在")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) GetTestResult(ctx *gin.Context) {
	taskID := ctx.Param("task_id")

	result, err := c.service.GetTestResult(taskID)
	if err != nil {
		utils.NotFoundResponse(ctx, "测试任务不存在")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) ScanInstances(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.ScanInstancesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.StartScanInstances(ctx.Request.Context(), id, req)
	if err != nil {
		utils.NotFoundResponse(ctx, "主机不存在")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) GetScanResult(ctx *gin.Context) {
	hostID := ctx.Param("id")
	taskID := ctx.Param("task_id")

	result, err := c.service.GetScanResult(taskID)
	if err != nil {
		utils.NotFoundResponse(ctx, "扫描任务不存在")
		return
	}
	if result.HostID != hostID {
		utils.NotFoundResponse(ctx, "扫描任务不属于该主机")
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HostController) RegisterScannedInstance(ctx *gin.Context) {
	hostID := ctx.Param("id")

	var req services.RegisterScannedInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	instanceID, err := c.service.RegisterScannedInstance(ctx.Request.Context(), hostID, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "登记实例失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"instance_id": instanceID, "message": "Instance registered"})
}

func (c *HostController) RegisterScannedInstances(ctx *gin.Context) {
	hostID := ctx.Param("id")

	var req services.BatchRegisterScannedInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.RegisterScannedInstances(ctx.Request.Context(), hostID, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "批量登记实例失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}
