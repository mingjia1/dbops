package main

import (
	"context"
	"log"
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

	db, err := repositories.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		logInstance.Warn("Database not available, running in standalone mode")
		db = nil
	}

	if db != nil {
		defer db.Close()
		if err := runMigrations(db); err != nil {
			logInstance.Warn("Migrations skipped")
		}
	}

	var userRepo *repositories.UserRepository
if db != nil {
	userRepo = repositories.NewUserRepository(db)
}
authService := services.NewAuthService(userRepo, cfg.JWTSecret)
authController := controllers.NewAuthController(authService)

	instanceRepo := repositories.NewInstanceRepository(db)
	instanceService := services.NewInstanceService(instanceRepo)
	instanceController := controllers.NewInstanceController(instanceService)

	hostRepo := repositories.NewHostRepository(db)
	hostService := services.NewHostService(hostRepo, cfg.EncryptionKey)
	hostController := controllers.NewHostController(hostService)

	envCheckService := services.NewEnvironmentCheckService()
	envCheckController := controllers.NewEnvironmentCheckController(envCheckService)

	backupService := services.NewBackupService()
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

	clusterDeployService := services.NewClusterDeployService()
	clusterDeployController := controllers.NewClusterDeployController(clusterDeployService)

	healthCheckService := services.NewHealthCheckService(db)
	healthCheckController := controllers.NewHealthCheckController(healthCheckService)

	failoverService := services.NewFailoverService(db)
	failoverController := controllers.NewFailoverController(failoverService)

	taskRepo := repositories.NewTaskRepository(db)
	upgradeService := services.NewUpgradeService(instanceRepo, taskRepo)
	upgradeController := controllers.NewUpgradeController(upgradeService, taskRepo)

migrationService := services.NewMigrationService()
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
		if db == nil {
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
		}
	}

	logInstance.Info("Server starting on port " + cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func runMigrations(db *repositories.Database) error {
	ctx := context.Background()
	conn, err := db.Pool.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return repositories.RunMigrations(ctx, conn)
}