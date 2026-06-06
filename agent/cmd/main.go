package main

import (
	"context"
	"log"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-agent/internal/executor"
	"github.com/monkeycode/mysql-ops-agent/internal/collector"
	"github.com/monkeycode/mysql-ops-agent/pkg/config"
	"github.com/monkeycode/mysql-ops-agent/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logInstance := logger.New(cfg.LogLevel)
	logInstance.Info("Starting MySQL Ops Agent")

	taskExecutor := executor.NewTaskExecutor()
	metricsCollector := collector.NewMetricsCollector()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code":    200,
			"message": "success",
			"data":    gin.H{"status": "ok", "service": "mysql-ops-agent"},
		})
	})

	// P1-2: 校验 platform-backend 调用 agent 时携带的 Bearer token, 与 config.agent_token 一致.
	authAgent := func(c *gin.Context) {
		if cfg.AgentToken == "" {
			c.AbortWithStatusJSON(500, gin.H{"code": 500, "message": "agent_token not configured"})
			return
		}
		h := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if len(h) < len(prefix) || h[:len(prefix)] != prefix {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "missing bearer token"})
			return
		}
		tok := h[len(prefix):]
		if tok != cfg.AgentToken {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "invalid agent token"})
			return
		}
		c.Next()
	}

	// 跳过授权的辅助: 把 agent 路由组挂上中间件.
	_ = authAgent

	agent := r.Group("/agent", authAgent)
	{
		tasks := agent.Group("/tasks")
		{
			tasks.POST("/deploy", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteDeploy(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Execution failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/backup", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteBackup(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Execution failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/restore", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteRestore(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Execution failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/health-check", func(c *gin.Context) {
				instanceID := c.Query("instance_id")
				result, err := taskExecutor.ExecuteHealthCheck(c.Request.Context(), instanceID)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Health check failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/migration", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteMigration(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Migration execution failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			// A1: 之前 backend 调 /agent/tasks/migration-verify 会 404,
			// 现在路由到 MigrationExecutor.VerifyMigration.
			tasks.POST("/migration-verify", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result := taskExecutor.ExecuteVerifyMigration(c.Request.Context(), req)
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			// A1: 之前 backend 调 /agent/tasks/upgrade 会 404.
			// 派发到 UpgradeExecutor 的具体方法, 通过 req.Config["upgrade_type"] 区分.
			tasks.POST("/upgrade", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				upgradeType, _ := req.Config["upgrade_type"].(string)
				ue := taskExecutor.UpgradeExecutor

				var result *executor.TaskResult
				var execErr error
				switch upgradeType {
				case "rolling":
					result, execErr = ue.ExecuteRollingUpgrade(c.Request.Context(), req)
				case "in-place", "":
					result, execErr = ue.ExecuteInPlaceUpgrade(c.Request.Context(), req)
				default:
					c.JSON(400, gin.H{"code": 400, "message": "unsupported upgrade_type: " + upgradeType})
					return
				}
				if execErr != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Upgrade failed: " + execErr.Error()})
					return
				}
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			// B8: 长任务进度查询. 当前 agent 端不持久化 task 状态 (内存),
			// 返 "no record" 让 backend 知道任务已结束. 后续可以挂 Redis 持久化.
			tasks.GET("/:id/progress", func(c *gin.Context) {
				taskID := c.Param("id")
				if taskID == "" {
					c.JSON(400, gin.H{"code": 400, "message": "task id required"})
					return
				}
				c.JSON(404, gin.H{
					"code":    404,
					"message": "agent has no in-memory record of task " + taskID + " (long ops don't track progress in v1; rely on backend TaskRepository)",
				})
			})

			tasks.POST("/migration-switch", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteMigrationSwitch(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Migration switch execution failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/cluster-switch", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				switchType, _ := req.Config["switch_type"].(string)
				if switchType == "" {
					c.JSON(400, gin.H{"code": 400, "message": "switch_type is required"})
					return
				}

				result, err := taskExecutor.ExecuteDeploy(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Switch execution failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/role-query", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteRoleQuery(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Role query failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/role-promote", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteRolePromote(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Role promote failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/role-demote", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteRoleDemote(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Role demote failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/role-replica-rebuild", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteRoleReplicaRebuild(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Role replica rebuild failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/metrics", func(c *gin.Context) {
				instanceID := c.Query("instance_id")
				metrics, err := metricsCollector.CollectMySQLMetrics(c.Request.Context(), instanceID)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Metrics collection failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": metrics})
			})
		}
	}

	go func() {
		ctx := context.Background()
		for {
			metrics, _ := metricsCollector.CollectSystemMetrics(ctx, "localhost")
			metricsCollector.ReportMetrics(ctx, cfg.PlatformURL, metrics)
			time.Sleep(10 * time.Second)
		}
	}()

	logInstance.Info("Agent starting on port " + cfg.AgentPort)
	if err := r.Run(":" + cfg.AgentPort); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}
}