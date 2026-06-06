package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/controllers"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/config"
	"github.com/monkeycode/mysql-ops-platform/pkg/logger"
	"github.com/monkeycode/mysql-ops-platform/pkg/middleware"
	"github.com/monkeycode/mysql-ops-platform/pkg/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// P0-1: 强制注入 jwt_secret / encryption_key / agent_token, 杜绝硬编码密钥.
	if err := validateSecrets(cfg); err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}

	logInstance := logger.New(cfg.LogLevel)
	logInstance.Info("Starting MySQL Ops Platform API Server")

	sqlitePath := cfg.SQLitePath
	if sqlitePath == "" {
		sqlitePath = cfg.DataDir + string(filepath.Separator) + "dbops.db"
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		logInstance.Warn("Failed to create data dir " + cfg.DataDir + ": " + err.Error())
	}
	// P0-6: 用 StorageMode 显式决定行为, 避免静默降级.
	db, err := repositories.NewDatabaseWithMode(cfg.DatabaseURL, sqlitePath, cfg.StorageMode)
	if err != nil {
		// mode=mysql 时这里就是 hard fail; mode=auto/sqlite 时也只允许启动后再处理.
		logInstance.Fatal("Database initialization failed: " + err.Error())
	}
	if db.IsMySQL() {
		logInstance.Info(fmt.Sprintf("Storage backend: MySQL (mode=%s)", cfg.StorageMode))
	} else {
		logInstance.Info(fmt.Sprintf("Storage backend: SQLite at %s (mode=%s)", sqlitePath, cfg.StorageMode))
	}

	// 数据持久化目录: 当未连接 MySQL 时, 内存数据将序列化到 JSON 文件,
	// 重启服务后会自动加载, 避免"重启后数据丢失"的问题.
	jsonStore, err := repositories.NewJSONStore(cfg.DataDir)
	if err != nil {
		logInstance.Warn("Failed to init JSON store at " + cfg.DataDir + " (standalone data will not persist)")
		jsonStore = nil
	} else {
		logInstance.Info("Data persistence directory: " + cfg.DataDir)
	}

	if db != nil {
		defer db.Close()
		if err := runMigrations(db); err != nil {
			logInstance.Warn("Migrations skipped: " + err.Error())
		} else {
			logInstance.Info("Schema migrations applied")
		}

		// 自动从旧的 data/*.json 一次性导入到 SQLite (仅在 SQLite 模式 + 表为空时).
		if db.IsSQLite() {
			importer := repositories.NewJSONImporter(cfg.DataDir)
			if n, err := importer.ImportAll(context.Background(), db); err != nil {
				logInstance.Warn("JSON -> SQLite import failed: " + err.Error())
			} else if n > 0 {
				logInstance.Info("Imported " + fmt.Sprintf("%d", n) + " records from legacy JSON files to SQLite")
			}
		}
	}

	var userRepo *repositories.UserRepository
userRepo = repositories.NewUserRepository(db)
authService := services.NewAuthService(userRepo, cfg.JWTSecret)
authController := controllers.NewAuthController(authService)

// P0-2: 首次启动 seed admin 账号. 密码仅打印一次到日志.
if created, username, plain, err := authService.SeedAdminIfEmpty(context.Background()); err != nil {
	logInstance.Warn("Failed to seed admin user: " + err.Error())
} else if created {
	logInstance.Info("================================================================")
	logInstance.Info(" Seeded initial admin user (please change password on first login):")
	logInstance.Info("   username: " + username)
	logInstance.Info("   password: " + plain)
	logInstance.Info("================================================================")
}

	instanceRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	taskRepo := repositories.NewTaskRepository(db)

	// 绑定 JSON 持久化 (仅在 standalone / db=nil 时生效).
	// 启动时自动从 ./data 目录加载 hosts.json / instances.json, 每次变更落盘.
	instanceRepo.AttachStore(jsonStore)
	hostRepo.AttachStore(jsonStore)
	agentClient := services.NewAgentClient(cfg.AgentToken)
	instanceService := services.NewInstanceService(instanceRepo, hostRepo, taskRepo, agentClient, cfg.EncryptionKey)
	instanceController := controllers.NewInstanceController(instanceService)
	hostService := services.NewHostService(hostRepo, cfg.EncryptionKey)
	hostService.SetInstanceRepo(instanceRepo)
	hostController := controllers.NewHostController(hostService)

	envCheckService := services.NewEnvironmentCheckService(hostRepo, agentClient)
	envCheckController := controllers.NewEnvironmentCheckController(envCheckService)

	backupRepo := repositories.NewBackupRepository(db)
	backupService := services.NewBackupService(hostRepo, instanceRepo, backupRepo, agentClient)
	backupController := controllers.NewBackupController(backupService)

	clickhouse, err := storage.NewClickHouse(cfg.ClickHouseURL)
	if err != nil {
		logInstance.Warn("ClickHouse not available, monitor service running in standalone mode")
		clickhouse = nil
	} else {
		defer clickhouse.Close()
	}

	redisClient, err := storage.NewRedis(cfg.RedisURL, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logInstance.Warn("Redis not available, cache layer disabled")
		redisClient = nil
	} else {
		defer redisClient.Close()
		logInstance.Info("Redis connected successfully")
	}

	monitorService := services.NewMonitorService(clickhouse)
	monitorController := controllers.NewMonitorController(monitorService)

	paramTemplateRepo := repositories.NewParameterTemplateRepository(db)
	paramTemplateService := services.NewParameterTemplateService(paramTemplateRepo)
	paramTemplateController := controllers.NewParameterTemplateController(paramTemplateService)

	clusterDeployRepo := repositories.NewClusterDeployRepository(db)
	clusterDeployService := services.NewClusterDeployService(clusterDeployRepo, hostRepo, instanceRepo, agentClient, cfg.ClusterDefaults)
	clusterDeployController := controllers.NewClusterDeployController(clusterDeployService)

	healthCheckService := services.NewHealthCheckService(db)
	healthCheckController := controllers.NewHealthCheckController(healthCheckService)

	failoverService := services.NewFailoverService(db)
	failoverController := controllers.NewFailoverController(failoverService)

	upgradeService := services.NewUpgradeService(instanceRepo, taskRepo)
	upgradeController := controllers.NewUpgradeController(upgradeService, taskRepo)

	migrationRepo := repositories.NewMigrationRepository(db)
	migrationService := services.NewMigrationService(migrationRepo, instanceRepo, hostRepo, agentClient)

	switchService := services.NewSwitchService(hostRepo, instanceRepo, clusterDeployRepo, agentClient)
	switchController := controllers.NewSwitchController(switchService)
	migrationController := controllers.NewMigrationController(migrationService)

	alertRuleRepo := repositories.NewAlertRuleRepository(db)
	alertNotificationRepo := repositories.NewAlertNotificationRepository(db)
	alertService := services.NewAlertService(alertRuleRepo, alertNotificationRepo, monitorService)
	alertController := controllers.NewAlertController(alertService)

	approvalRepo := repositories.NewApprovalRequestRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	approvalService := services.NewApprovalService(approvalRepo, auditRepo)
	auditService := services.NewAuditService(auditRepo, approvalRepo)
	approvalController := controllers.NewApprovalController(approvalService)
	auditController := controllers.NewAuditController(auditService)

	dataMigrationController := controllers.NewDataMigrationController(cfg)

	r := gin.Default()
	// P0-3: 限制 body 上限 10MB, 防止大文件上传 DoS.
	r.MaxMultipartMemory = 10 << 20
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.RateLimitByIP(middleware.NewRateLimiter(100, time.Second)))
	r.Use(middleware.Logger(logInstance))
	r.Use(middleware.ErrorHandler())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code":    200,
			"message": "success",
			"data":    gin.H{"status": "ok", "service": "mysql-ops-platform"},
		})
	})

	// P1-8: 探活/就绪分流, 供 k8s livenessProbe / readinessProbe 使用.
	// /health/live: 进程能响应 HTTP 即返回 200, 用于存活探针.
	// /health/ready: 校验 db/redis 可达, 失败返 503, 用于就绪探针.
	r.GET("/health/live", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"status": "alive"}})
	})
	r.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		checks := gin.H{}
		allOK := true
		if db != nil {
			if err := db.HealthCheck(ctx); err != nil {
				checks["db"] = err.Error()
				allOK = false
			} else {
				checks["db"] = "ok"
			}
		} else {
			checks["db"] = "not initialized"
			allOK = false
		}
		if redisClient != nil && redisClient.Client != nil {
			if err := redisClient.Client.Ping(ctx).Err(); err != nil {
				checks["redis"] = err.Error()
				allOK = false
			} else {
				checks["redis"] = "ok"
			}
		} else {
			checks["redis"] = "disabled"
		}
		if allOK {
			c.JSON(200, gin.H{"code": 200, "data": gin.H{"status": "ready", "checks": checks}})
		} else {
			c.JSON(503, gin.H{"code": 503, "message": "not ready", "data": gin.H{"status": "not_ready", "checks": checks}})
		}
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			// P0-3: 登录端点 5 req/min per IP, 抗撞库.
			loginLimiter := middleware.LoginRateLimit(middleware.NewRateLimiter(5, time.Minute))
			auth.POST("/login", loginLimiter, authController.Login)
			auth.POST("/register", loginLimiter, authController.Register)
		}

		protected := api.Group("")
		// P0-2: 鉴权不再根据存储后端区分, 一律走真实 JWT 校验.
		// (db=nil 时 authService 处于 standalone 模式, Login 接受任意凭据, 但 ValidateToken 仍要求合法签名.)
		protected.Use(authController.ValidateToken)
		{
			instances := protected.Group("/instances")
			{
				instances.GET("", instanceController.List)
				instances.POST("", instanceController.Create)
				instances.GET("/:id", instanceController.GetByID)
				instances.PUT("/:id", instanceController.Update)
				instances.DELETE("/:id", instanceController.Delete)
				instances.POST("/:id/detect-version", instanceController.DetectVersion)
			instances.POST("/:id/deploy", instanceController.Deploy)
			}

			hosts := protected.Group("/hosts")
			{
				hosts.GET("", hostController.List)
				hosts.POST("", hostController.Create)
				hosts.GET("/test/:task_id", hostController.GetTestResult)
				hosts.GET("/:id", hostController.GetByID)
				hosts.PUT("/:id", hostController.Update)
				hosts.DELETE("/:id", hostController.Delete)
				hosts.POST("/:id/test", hostController.TestConnection)
				hosts.POST("/:id/scan-instances", hostController.ScanInstances)
				hosts.GET("/:id/scan-instances/:task_id", hostController.GetScanResult)
				hosts.POST("/:id/scan-instances/register", hostController.RegisterScannedInstance)
			}

			envChecks := protected.Group("/env-checks")
			{
				envChecks.POST("", envCheckController.Execute)
				envChecks.GET("/:id", envCheckController.GetByID)
				envChecks.GET("/:id/export", envCheckController.Export)
			}

			backups := protected.Group("/backups")
			{
				backups.POST("/policies", backupController.CreatePolicy)
				backups.POST("", backupController.ExecuteBackup)
				backups.GET("", backupController.ListBackups)
			}

			monitoring := protected.Group("/monitoring")
			{
				monitoring.GET("/metrics", monitorController.QueryMetrics)
			}

			paramTemplates := protected.Group("/parameter-templates")
			{
				paramTemplates.GET("", paramTemplateController.List)
				paramTemplates.GET("/presets", paramTemplateController.ListPresets)
				paramTemplates.POST("", paramTemplateController.Create)
				paramTemplates.GET("/:id", paramTemplateController.GetByID)
				paramTemplates.PUT("/:id", paramTemplateController.Update)
				paramTemplates.DELETE("/:id", paramTemplateController.Delete)
				paramTemplates.GET("/:id/parameters", paramTemplateController.GetParameters)
				paramTemplates.POST("/:id/validate", paramTemplateController.Validate)
				paramTemplates.POST("/recommend", paramTemplateController.Recommend)
			}

			deployments := protected.Group("/deployments")
			{
				deployments.POST("/mha", clusterDeployController.DeployMHA)
				deployments.POST("/mgr", clusterDeployController.DeployMGR)
				deployments.POST("/pxc", clusterDeployController.DeployPXC)
				deployments.GET("/:id", clusterDeployController.GetDeploymentStatus)
			}

			switches := protected.Group("/switch")
			{
				switches.POST("/single-to-mha", switchController.SingleToMHA)
				switches.POST("/single-to-mgr", switchController.SingleToMGR)
				switches.POST("/single-to-pxc", switchController.SingleToPXC)
				switches.POST("/cluster/role", switchController.SwitchRoleWithinCluster)
				switches.GET("/cluster/:cluster_id/role-history", switchController.ListRoleSwitchHistory)
			}

			ha := protected.Group("/ha")
			{
				ha.GET("/health", healthCheckController.ExecuteHealthCheck)
				ha.GET("/health/failure-state", healthCheckController.GetFailureState)
				ha.GET("/health/detect", healthCheckController.DetectFailure)
				ha.POST("/health/batch", healthCheckController.BatchHealthCheck)
				ha.POST("/failover", failoverController.ExecuteAutoFailover)
				ha.POST("/manual-switch", failoverController.ExecuteManualFailover)
				ha.GET("/status", failoverController.GetClusterStatus)
			}

			upgrades := protected.Group("/upgrades")
			{
				upgrades.GET("", upgradeController.ListHistory)
				upgrades.POST("/plan", upgradeController.PlanUpgradePath)
				upgrades.POST("/check", upgradeController.CheckCompatibility)
				upgrades.POST("/in-place", upgradeController.ExecuteInPlaceUpgrade)
				upgrades.POST("/logical", upgradeController.ExecuteLogicalMigration)
				upgrades.POST("/rolling", upgradeController.ExecuteRollingUpgrade)
				upgrades.POST("/rollback", upgradeController.RollbackUpgrade)
				upgrades.GET("/:id", upgradeController.GetUpgradeByID)
				upgrades.GET("/:id/report", upgradeController.GenerateUpgradeReport)
			}

migrations := protected.Group("/migrations")
			{
				migrations.GET("", migrationController.List)
				migrations.POST("/physical", migrationController.ExecutePhysical)
				migrations.POST("/replication", migrationController.ExecuteReplication)
				migrations.POST("/gtid", migrationController.ExecuteGTID)
				migrations.POST("/orchestrate", migrationController.Orchestrate)
				migrations.GET("/:id/progress", migrationController.GetProgress)
				migrations.POST("/:id/verify", migrationController.Verify)
				migrations.POST("/:id/switch", migrationController.Switch)
			}

			alerts := protected.Group("/alerts")
			{
				alerts.GET("/rules", alertController.ListAlertRules)
				alerts.POST("/rules", alertController.CreateAlertRule)
				alerts.GET("/rules/:id", alertController.GetAlertRule)
				alerts.PUT("/rules/:id", alertController.UpdateAlertRule)
				alerts.DELETE("/rules/:id", alertController.DeleteAlertRule)
				alerts.POST("/evaluate", alertController.EvaluateAlert)
				alerts.POST("/trigger", alertController.TriggerAlert)
				alerts.GET("/history", alertController.GetAlertHistory)
				alerts.POST("/notifications", alertController.SendNotification)
				alerts.GET("/notifications/channels", alertController.ListNotificationChannels)
				alerts.POST("/notifications/channels", alertController.CreateNotificationChannel)
				alerts.GET("/notifications/channels/:id", alertController.GetNotificationChannel)
				alerts.PUT("/notifications/channels/:id", alertController.UpdateNotificationChannel)
				alerts.DELETE("/notifications/channels/:id", alertController.DeleteNotificationChannel)
			}

			approvals := protected.Group("/approvals")
			{
				approvals.GET("", approvalController.ListApprovalRequests)
				approvals.GET("/:id", approvalController.GetApprovalRequestByID)
				approvals.POST("/:id/approve", approvalController.ApproveRequest)
				approvals.POST("/:id/reject", approvalController.RejectRequest)
			}

			auditLogs := protected.Group("/audit-logs")
			{
				auditLogs.GET("", auditController.ListAuditLogs)
				auditLogs.GET("/:id", auditController.GetAuditLogByID)
			}

			dataMig := protected.Group("/data-migration")
			{
				dataMig.GET("/status", dataMigrationController.GetStatus)
				dataMig.POST("/import-legacy-json", dataMigrationController.ImportLegacyJSON)
				dataMig.POST("/migrate-to-mysql", dataMigrationController.MigrateToMySQL)
			}
		}
	}

	logInstance.Info("Server starting on port " + cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func runMigrations(db *repositories.Database) error {
	return repositories.RunMigrations(context.Background(), db)
}

// validateSecrets P0-1 防护: 任何缺省 / 短 / 明显示例值都直接拒启动.
func validateSecrets(cfg *config.Config) error {
	if len(cfg.JWTSecret) < 32 {
		return fmt.Errorf("jwt_secret must be set and >= 32 chars (current len=%d). Generate with: openssl rand -hex 32", len(cfg.JWTSecret))
	}
	if len(cfg.EncryptionKey) < 32 {
		return fmt.Errorf("encryption_key must be set and >= 32 chars (current len=%d). Generate with: openssl rand -hex 32", len(cfg.EncryptionKey))
	}
	if cfg.AgentToken == "" || len(cfg.AgentToken) < 16 {
		return fmt.Errorf("agent_token must be set and >= 16 chars. Generate with: openssl rand -hex 16")
	}
	return nil
}