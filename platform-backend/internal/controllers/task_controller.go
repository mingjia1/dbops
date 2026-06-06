package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

// TaskController B8: 暴露 GET /api/v1/tasks/:id, 让前端轮询长任务 (升级/备份/迁移) 进度.
// 之前没有这个端点, 前端只能盯在 agent_client 30s timeout 后的 5xx 错.
type TaskController struct {
	taskRepo *repositories.TaskRepository
}

func NewTaskController(taskRepo *repositories.TaskRepository) *TaskController {
	return &TaskController{taskRepo: taskRepo}
}

// GetByID 返回 task 当前状态, 找不到返 404.
func (c *TaskController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "task id required"})
		return
	}
	task, err := c.taskRepo.GetByID(ctx.Request.Context(), id)
	if err != nil || task == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "task not found"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    task,
	})
}

// ListByInstance 给前端展示某实例的所有任务历史.
func (c *TaskController) ListByInstance(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
	limit := 50
	if v := ctx.Query("limit"); v != "" {
		// 简单解析, 失败 fallback 50
		var n int
		for _, c := range v {
			if c < '0' || c > '9' {
				n = -1
				break
			}
			n = n*10 + int(c-'0')
		}
		if n > 0 && n <= 200 {
			limit = n
		}
	}
	tasks, err := c.taskRepo.List(ctx.Request.Context(), instanceID, limit, 0)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to list tasks"})
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    tasks,
	})
}
