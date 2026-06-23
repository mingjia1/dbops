package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-agent/internal/collector"
	"github.com/monkeycode/mysql-ops-agent/internal/executor"
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
	if cfg.Relay.Enabled && cfg.Relay.RelayHost != "" {
		taskExecutor.SetRelayConfig(cfg.Relay.RelayHost, cfg.Relay.RelayPort, cfg.Relay.RelayToken)
	}
	metricsCollector := collector.NewMetricsCollector()
	environmentChecker := executor.NewEnvironmentChecker()
	accountManager := executor.NewAccountManager()

	relayManager := executor.NewRelayPackageManager(cfg.Relay.CacheDir, cfg.Relay.MaxCacheSizeGB, cfg.Relay.CacheExpireHours)

	var toolInstaller *executor.ToolInstaller
	if cfg.Relay.Enabled && cfg.Relay.RelayHost != "" {
		toolInstaller = executor.NewToolInstallerWithRelay(cfg.Relay.RelayHost, cfg.Relay.RelayPort, cfg.Relay.RelayToken)
	} else {
		toolInstaller = executor.NewToolInstaller()
	}

	haInstaller := executor.NewHAInstaller()

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

	compatAccounts := r.Group("/api/v1/accounts")
	{
		compatAccounts.POST("/setup", func(c *gin.Context) {
			var req executor.AccountSetupRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
				return
			}
			ctx := c.Request.Context()
			host := req.Host
			if host == "" { host = "127.0.0.1" }
			port := req.Port
			if port == 0 { port = 3306 }
			adminUser := req.AdminUser
			if adminUser == "" { adminUser = "root" }
			if req.RootPassword != "" {
				if err := accountManager.SetupRootAccount(ctx, host, port, req.RootPassword); err != nil {
					c.JSON(500, gin.H{"code": 500, "message": err.Error()})
					return
				}
			}
			if req.ReplUser != "" && req.ReplPass != "" {
				if err := accountManager.SetupReplAccount(ctx, host, port, adminUser, req.AdminPass, req.ReplUser, req.ReplPass); err != nil {
					c.JSON(500, gin.H{"code": 500, "message": err.Error()})
					return
				}
			}
			if req.MonitorUser != "" && req.MonitorPass != "" {
				if err := accountManager.SetupMonitorAccount(ctx, host, port, adminUser, req.AdminPass, req.MonitorUser, req.MonitorPass); err != nil {
					c.JSON(500, gin.H{"code": 500, "message": err.Error()})
					return
				}
			}
			c.JSON(200, gin.H{"code": 200, "message": "Accounts setup completed"})
		})
		compatAccounts.POST("/rotate", func(c *gin.Context) {
			var req executor.AccountRotateRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
				return
			}
			if req.Host == "" { req.Host = "127.0.0.1" }
			if req.Port == 0 { req.Port = 3306 }
			if req.TargetUser == "" || req.NewPassword == "" {
				c.JSON(400, gin.H{"code": 400, "message": "target_user and new_password are required"})
				return
			}
			if err := accountManager.RotatePassword(c.Request.Context(), req.Host, req.Port, req.AdminUser, req.AdminPass, req.TargetUser, req.NewPassword); err != nil {
				c.JSON(500, gin.H{"code": 500, "message": err.Error()})
				return
			}
			c.JSON(200, gin.H{"code": 200, "message": "Password rotated"})
		})
	}

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

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
				defer cancel()
				result, err := taskExecutor.ExecuteDeploy(ctx, req)
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

			tasks.POST("/backup-scan", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteBackupScan(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Backup scan failed"})
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

			tasks.POST("/instance-admin", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteInstanceAdmin(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Instance admin failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			tasks.POST("/health-check", func(c *gin.Context) {
				req := executor.DeployTaskRequest{InstanceID: c.Query("instance_id")}
				if c.Request.ContentLength != 0 {
					if err := c.ShouldBindJSON(&req); err != nil {
						c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
						return
					}
				}
				result, err := taskExecutor.ExecuteHealthCheck(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Health check failed"})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			// P0: backend DetectVersion 之前返 error, 阻断升级链路.
			// 现在调 agent 真跑 SELECT @@version, @@version_comment.
			tasks.POST("/version-detect", func(c *gin.Context) {
				var req executor.DeployTaskRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result, err := taskExecutor.ExecuteVersionDetect(c.Request.Context(), req)
				if err != nil {
					log.Printf("[ERROR] Version detect failed: %v", err)
					c.JSON(500, gin.H{"code": 500, "message": "Version detect failed"})
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
				case "logical":
					// P0: 之前 logical 走 default → 400 "unsupported upgrade_type",
					// 阻断了 backend upgrade_service.ExecuteLogicalMigration 整条路径.
					result, execErr = ue.ExecuteLogicalMigration(c.Request.Context(), req)
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

				result, err := taskExecutor.ExecuteClusterSwitch(c.Request.Context(), req)
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

			// 工具安装端点
			tasks.POST("/install-tools", func(c *gin.Context) {
				var req executor.InstallToolsRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result := toolInstaller.InstallTools(c.Request.Context(), req)
				statusCode := 200
				if result.Status == "failed" {
					statusCode = 500
				}

				c.JSON(statusCode, gin.H{"code": statusCode, "message": result.Message, "data": result})
			})

			// 空白主机初始化端点 (从空白主机到可运行的MySQL实例)
			tasks.POST("/blank-host-init", func(c *gin.Context) {
				var req executor.BlankHostInitRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result := toolInstaller.InitBlankHost(c.Request.Context(), req)
				statusCode := 200
				if result.Status == "failed" {
					statusCode = 500
				}

				c.JSON(statusCode, gin.H{"code": statusCode, "message": result.Message, "data": result})
			})

			// 通用集群初始化端点 (从空白主机到集群部署)
			tasks.POST("/general-cluster-init", func(c *gin.Context) {
				var req executor.GeneralClusterInitRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result := toolInstaller.InitGeneralCluster(c.Request.Context(), req)
				statusCode := 200
				if result.Status == "failed" {
					statusCode = 500
				}

				c.JSON(statusCode, gin.H{"code": statusCode, "message": result.Message, "data": result})
			})

			// 检查工具可用性端点
			tasks.GET("/check-tools", func(c *gin.Context) {
				tools := []string{"mysql", "mysqld", "xtrabackup"}
				result := toolInstaller.CheckToolsAvailable(tools)
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			// 环境检查端点
			tasks.POST("/check-environment", func(c *gin.Context) {
				var req executor.EnvironmentCheckRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				result := environmentChecker.CheckEnvironment(c.Request.Context(), req)
				statusCode := 200
				if result.Status == "failed" {
					statusCode = 500
				}

				c.JSON(statusCode, gin.H{"code": statusCode, "message": result.Message, "data": result})
			})

			// 账号管理端点：初始化账号（root/repl/monitor）
			tasks.POST("/account-setup", func(c *gin.Context) {
				var req executor.AccountSetupRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				ctx := c.Request.Context()
				host := req.Host
				if host == "" {
					host = "127.0.0.1"
				}
				port := req.Port
				if port == 0 {
					port = 3306
				}
				adminUser := req.AdminUser
				if adminUser == "" {
					adminUser = "root"
				}

				// Setup root account
				if req.RootPassword != "" {
					if err := accountManager.SetupRootAccount(ctx, host, port, req.RootPassword); err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "Root account setup failed: " + err.Error()})
						return
					}
				}

				// Setup replication account
				if req.ReplUser != "" && req.ReplPass != "" {
					if err := accountManager.SetupReplAccount(ctx, host, port, adminUser, req.AdminPass, req.ReplUser, req.ReplPass); err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "Replication account setup failed: " + err.Error()})
						return
					}
				}

				// Setup monitoring account
				if req.MonitorUser != "" && req.MonitorPass != "" {
					if err := accountManager.SetupMonitorAccount(ctx, host, port, adminUser, req.AdminPass, req.MonitorUser, req.MonitorPass); err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "Monitor account setup failed: " + err.Error()})
						return
					}
				}

				c.JSON(200, gin.H{"code": 200, "message": "Accounts setup completed successfully"})
			})

			// 账号管理端点：密码轮转
			tasks.POST("/account-rotate", func(c *gin.Context) {
				var req executor.AccountRotateRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				if req.Host == "" {
					req.Host = "127.0.0.1"
				}
				if req.Port == 0 {
					req.Port = 3306
				}
				if req.TargetUser == "" || req.NewPassword == "" {
					c.JSON(400, gin.H{"code": 400, "message": "target_user and new_password are required"})
					return
				}

				if err := accountManager.RotatePassword(c.Request.Context(), req.Host, req.Port, req.AdminUser, req.AdminPass, req.TargetUser, req.NewPassword); err != nil {
					c.JSON(500, gin.H{"code": 500, "message": "Password rotation failed: " + err.Error()})
					return
				}

				c.JSON(200, gin.H{"code": 200, "message": "Password rotated successfully"})
			})
		}

		relay := agent.Group("/relay")
		{
			relay.GET("/status", func(c *gin.Context) {
				status := relayManager.GetStatus()
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": status})
			})

			relay.GET("/packages", func(c *gin.Context) {
				pkgs := relayManager.ListPackages()
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": pkgs})
			})

			relay.POST("/fetch", func(c *gin.Context) {
				var req executor.RelayPackageRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}
				result, err := relayManager.FetchAndCache(c.Request.Context(), req)
				if err != nil {
					log.Printf("[ERROR] Relay fetch failed: %v", err)
					c.JSON(500, gin.H{"code": 500, "message": "Fetch failed"})
					return
				}
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})

			relay.GET("/packages/download", func(c *gin.Context) {
				name := c.Query("name")
				if name == "" {
					c.JSON(400, gin.H{"code": 400, "message": "Missing package name"})
					return
				}
				filePath, err := relayManager.GetPackagePath(name)
				if err != nil || filePath == "" {
					c.JSON(404, gin.H{"code": 404, "message": "Package not found"})
					return
				}
				c.File(filePath)
			})

			relay.POST("/prefetch", func(c *gin.Context) {
				var req executor.RelayPrefetchRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}
				results, err := relayManager.PrefetchPackages(c.Request.Context(), req)
				if err != nil {
					log.Printf("[ERROR] Prefetch failed: %v", err)
					c.JSON(500, gin.H{"code": 500, "message": "Prefetch failed"})
					return
				}
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": results})
			})
		}

		ha := agent.Group("/ha-install")
		{
			ha.POST("/replication", func(c *gin.Context) {
				var req executor.HAInstallRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}
				result, err := haInstaller.InstallReplicationSetup(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": err.Error()})
					return
				}
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})
			ha.POST("/mha", func(c *gin.Context) {
				var req executor.HAInstallRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}
				result, err := haInstaller.InstallMHA(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": err.Error()})
					return
				}
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})
			ha.POST("/mgr", func(c *gin.Context) {
				var req executor.HAInstallRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}
				result, err := haInstaller.InstallMGR(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": err.Error()})
					return
				}
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})
			ha.POST("/pxc", func(c *gin.Context) {
				var req executor.HAInstallRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
					return
				}
				result, err := haInstaller.InstallPXC(c.Request.Context(), req)
				if err != nil {
					c.JSON(500, gin.H{"code": 500, "message": err.Error()})
					return
				}
				c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
			})
		}
	}

	// P0: 之前这段 goroutine 每 10s 调 ReportMetrics, 但 ReportMetrics
	// 只是个 fmt.Printf noop, 假数据 + 不发 backend = 永远零真实指标.
	// 修: 暂时禁用整段, 真接入 backend 推送留待下轮 (需先有 backend ingest 端点).
	// go func() {
	// 	ctx := context.Background()
	// 	for {
	// 		_ = metricsCollector.CollectSystemMetrics
	// 		time.Sleep(10 * time.Second)
	// 	}
	// }()
	_ = metricsCollector // 保留引用避免 unused 警告

	// P0: 用 http.Server 替代 gin.Default 的 r.Run, 设 ReadHeaderTimeout 等.
	// gin.Default 底层 http.Server 全 0 timeout, Slowloris 攻击可 pin 死端口.
	r.Use(func(c *gin.Context) {
		// 透传 backend X-Trace-Id; 没有就自己生成一个, 至少 agent 端有 trace.
		tid := c.GetHeader("X-Trace-Id")
		if tid == "" {
			tid = "agent-" + c.Request.Header.Get("X-Request-Id")
		}
		c.Set("trace_id", tid)
		c.Writer.Header().Set("X-Trace-Id", tid)
		c.Next()
	})

	logInstance.Info("Agent starting on port " + cfg.AgentPort)
	srv := &http.Server{
		Addr:              ":" + cfg.AgentPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Minute, // normal API timeout; long tasks use background context
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}
}
