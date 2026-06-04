package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	logInstance := logger.New(cfg.LogLevel)
	logInstance.Info("Starting MySQL Ops Platform API Server")

	sqlitePath := cfg.SQLitePath
	if sqlitePath == "" {
		sqlitePath = cfg.DataDir + string(filepath.Separator) + "dbops.db"
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		logInstance.Warn("Failed to create data dir " + cfg.DataDir + ": " + err.Error())
	}
	db, err := repositories.NewDatabase(cfg.DatabaseURL, sqlitePath)
	if err != nil {
		logInstance.Warn("Database not available, running in standalone mode")
		db = nil
	}
	if db != nil {
		if db.IsMySQL() {
			logInstance.Info("Using MySQL as primary store")
		} else {
			logInstance.Info("Using SQLite as primary store at " + sqlitePath)
		}
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
		importer := repositories.NewJSONImporter(cfg.DataDir)
		if n, err := importer.ImportAll(context.Background(), db); err != nil {
			logInstance.Warn("JSON -> SQLite import failed: " + err.Error())
		} else if n > 0 {
			logInstance.Info("Imported " + fmt.Sprintf("%d", n) + " records from legacy JSON files to SQLite")
		}
	}

	var userRepo *repositories.UserRepository
if db != nil {
	userRepo = repositories.NewUserRepository(db)
}
authService := services.NewAuthService(userRepo, cfg.JWTSecret)
authController := controllers.NewAuthController(authService)

	instanceRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	taskRepo := repositories.NewTaskRepository(db)

	// 绑定 JSON 持久化 (仅在 standalone / db=nil 时生效).
	// 启动时自动从 ./data 目录加载 hosts.json / instances.json, 每次变更落盘.
	instanceRepo.AttachStore(jsonStore)
	hostRepo.AttachStore(jsonStore)
	agentClient := services.NewAgentClient()
	instanceService := services.NewInstanceService(instanceRepo, hostRepo, taskRepo, agentClient)
	instanceController := controllers.NewInstanceController(instanceService)
	hostService := services.NewHostService(hostRepo, cfg.EncryptionKey)
	hostService.SetInstanceRepo(instanceRepo)
	hostController := controllers.NewHostController(hostService)

	envCheckService := services.NewEnvironmentCheckService(hostRepo, agentClient)
	envCheckController := controllers.NewEnvironmentCheckController(envCheckService)

	backupService := services.NewBackupService(hostRepo, instanceRepo, agentClient)
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
	clusterDeployService := services.NewClusterDeployService(clusterDeployRepo, hostRepo, instanceRepo, agentClient)
	clusterDeployController := controllers.NewClusterDeployController(clusterDeployService)

	healthCheckService := services.NewHealthCheckService(db)
	healthCheckController := controllers.NewHealthCheckController(healthCheckService)

	failoverService := services.NewFailoverService(db)
	failoverController := controllers.NewFailoverController(failoverService)

	upgradeService := services.NewUpgradeService(instanceRepo, taskRepo)
	upgradeController := controllers.NewUpgradeController(upgradeService, taskRepo)

	migrationRepo := repositories.NewMigrationRepository(db)
	migrationService := services.NewMigrationService(migrationRepo, instanceRepo, agentClient)

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
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logInstance))
	r.Use(middleware.ErrorHandler())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code":    200,
			"message": "success",
			"data":    gin.H{"status": "ok", "service": "mysql-ops-platform"},
		})
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", authController.Login)
			auth.POST("/register", authController.Register)
		}

		protected := api.Group("")
		// SQLite 兜底时, 用户表为空, 用测试身份注入方便 dev 使用
		if db == nil || db.IsSQLite() {
			protected.Use(func(c *gin.Context) {
				c.Set("user_id", "test-user-001")
				c.Set("username", "testuser")
				c.Set("role", "admin")
				c.Next()
			})
		} else {
			protected.Use(authController.ValidateToken)
		}
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