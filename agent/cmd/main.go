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

	agent := r.Group("/agent")
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