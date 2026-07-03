package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/controllers"
	"github.com/jackcode/mysql-ops-platform/internal/plugins"
	pluginArch "github.com/jackcode/mysql-ops-platform/internal/plugins/arch"
	pluginMiddleware "github.com/jackcode/mysql-ops-platform/internal/plugins/middleware"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/aiprovider"
	"github.com/jackcode/mysql-ops-platform/pkg/config"
	"github.com/jackcode/mysql-ops-platform/pkg/logger"
	"github.com/jackcode/mysql-ops-platform/pkg/middleware"
	"github.com/jackcode/mysql-ops-platform/pkg/storage"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
	"go.uber.org/zap"
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

	// Force use old database file
	sqlitePath := cfg.SQLitePath
	if sqlitePath == "" {
		sqlitePath = "../db/dbops.db"
	}
	logInstance.Info("Forcing SQLite path: " + sqlitePath)

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
	roleRepo := repositories.NewRoleRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	auditService := services.NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))
	auditService.SetHMACSecret(cfg.EncryptionKey)
	authService := services.NewAuthService(userRepo, cfg.JWTSecret, auditService)
	authService.SetRoleRepository(roleRepo)
	authController := controllers.NewAuthController(authService)
	userService := services.NewUserService(userRepo)
	userService.SetRoleRepository(roleRepo)
	userService.SetAuditService(auditService)
	userController := controllers.NewUserController(userService)
	roleService := services.NewRoleService(roleRepo)
	roleService.SetAuditService(auditService)
	roleController := controllers.NewRoleController(roleService)

	if err := roleRepo.SeedBuiltinRoles(context.Background()); err != nil {
		logInstance.Warn("Failed to seed builtin roles: " + err.Error())
	}
	seedAuthSettings(context.Background(), db, logInstance)

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

	pluginRegistry := plugins.NewRegistry()
	agentCaller := func(ctx context.Context, host string, port int, path string, payload map[string]interface{}) (map[string]interface{}, error) {
		result, err := agentClient.CallAgentRaw(ctx, host, port, path, payload)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	_ = pluginRegistry.Register(pluginArch.NewReplicaAddonPlugin(agentCaller))
	_ = pluginRegistry.Register(pluginArch.NewMHAAddonPlugin(agentCaller))
	_ = pluginRegistry.Register(pluginArch.NewMGRAddonPlugin(agentCaller))
	_ = pluginRegistry.Register(pluginArch.NewPXCGaleraAddonPlugin(agentCaller))
	_ = pluginRegistry.Register(pluginMiddleware.NewKeepalivedAddonPlugin(agentCaller))
	_ = pluginRegistry.Register(pluginMiddleware.NewProxySQLAddonPlugin(agentCaller))
	// P0: 提前建 auditService, 让 instanceService/hostService 等可注入.
	instanceService := services.NewInstanceService(instanceRepo, hostRepo, taskRepo, agentClient, auditService, cfg.EncryptionKey)
	instanceController := controllers.NewInstanceController(instanceService)
	hostService := services.NewHostService(hostRepo, cfg.EncryptionKey, cfg.AgentToken, cfg.DataDir)
	hostService.SetInstanceRepo(instanceRepo)
	hostService.SetAgentClient(agentClient)
	hostController := controllers.NewHostController(hostService)

	envCheckService := services.NewEnvironmentCheckService(hostRepo, agentClient, cfg.EncryptionKey)
	envCheckController := controllers.NewEnvironmentCheckController(envCheckService)

	backupRepo := repositories.NewBackupRepository(db)
	backupService := services.NewBackupService(hostRepo, instanceRepo, backupRepo, agentClient, cfg.EncryptionKey, auditService, taskRepo)
	backupController := controllers.NewBackupController(backupService)
	instanceService.SetBackupService(backupService)

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
	paramTemplateService := services.NewParameterTemplateService(paramTemplateRepo, instanceService)
	paramTemplateController := controllers.NewParameterTemplateController(paramTemplateService)

	clusterDeployRepo := repositories.NewClusterDeployRepository(db)
	clusterDeployNodeRepo := repositories.NewClusterDeployNodeRepository(db)
	instanceService.SetDeployRepo(clusterDeployRepo)
	clusterDeployService := services.NewClusterDeployService(clusterDeployRepo, clusterDeployNodeRepo, hostRepo, instanceRepo, agentClient, cfg.ClusterDefaults, auditService)
	clusterDeployService.SetEncryptionKey(cfg.EncryptionKey)
	clusterDeployService.SetBackupService(backupService)
	clusterDeployService.SetHostService(hostService)

	pluginExec := plugins.NewExecutor(pluginRegistry)
	credentialRepo := repositories.NewCredentialRepository(db)
	credentialVault := services.NewCredentialVault(credentialRepo, cfg.EncryptionKey)
	clusterDeployService.SetCredentialVault(credentialVault)
	rebuildSvc := services.NewRebuildService(pluginExec, credentialVault, instanceRepo)
	clusterDeployService.SetRebuildService(rebuildSvc)
	deployOrchestrator := services.NewDeployOrchestrator(pluginRegistry, credentialVault, instanceRepo)
	scaleSvc := services.NewScaleService(deployOrchestrator, instanceRepo, pluginExec)
	clusterDeployService.SetScaleService(scaleSvc)

	settingsRepo := repositories.NewPlatformSettingsRepository(db)
	settingsController := controllers.NewSettingsController(settingsRepo)
	clusterDeployController := controllers.NewClusterDeployController(clusterDeployService)
	topologyService := services.NewTopologyService(instanceRepo)
	topologyController := controllers.NewTopologyController(topologyService)

	healthCheckService := services.NewHealthCheckService(db, cfg.EncryptionKey)
	healthCheckController := controllers.NewHealthCheckController(healthCheckService, instanceService)

	failoverService := services.NewFailoverService(db, cfg.EncryptionKey)
	failoverController := controllers.NewFailoverController(failoverService)

	upgradeService := services.NewUpgradeService(instanceRepo, taskRepo, agentClient, auditService)
	upgradeService.SetEncryptionKey(cfg.EncryptionKey)
	upgradeController := controllers.NewUpgradeController(upgradeService, taskRepo)

	versionCatalog := services.NewVersionCatalog()
	versionController := controllers.NewVersionController(versionCatalog)
	clusterDeployService.SetVersionCatalog(versionCatalog)

	migrationRepo := repositories.NewMigrationRepository(db)
	migrationService := services.NewMigrationService(migrationRepo, instanceRepo, hostRepo, agentClient, auditService)

	switchHistoryRepo := repositories.NewRoleSwitchHistoryRepository(db)
	switchService := services.NewSwitchService(hostRepo, instanceRepo, clusterDeployRepo, agentClient, switchHistoryRepo, auditService)
	switchService.SetEncryptionKey(cfg.EncryptionKey)
	switchController := controllers.NewSwitchController(switchService)
	failoverService.SetSwitchService(switchService)
	migrationController := controllers.NewMigrationController(migrationService)

	alertRuleRepo := repositories.NewAlertRuleRepository(db)
	alertNotificationRepo := repositories.NewAlertNotificationRepository(db)
	alertTemplateRepo := repositories.NewAlertTemplateRepository(db)
	escRepo := repositories.NewEscalationRepository(db)
	silenceRepo := repositories.NewSilenceRepository(db)
	inspectionTplRepo := repositories.NewInspectionTemplateRepository(db)
	inspectionRptRepo := repositories.NewInspectionReportRepository(db)
	alertService := services.NewAlertService(alertRuleRepo, alertNotificationRepo, monitorService)
	alertService.WithTemplateRepo(alertTemplateRepo).WithEscalationRepo(escRepo).WithSilenceRepo(silenceRepo)
	alertService.WithInspectionRepos(inspectionTplRepo, inspectionRptRepo)
	alertController := controllers.NewAlertController(alertService)
	alertTemplateController := controllers.NewAlertTemplateController(alertService)
	escalationController := controllers.NewEscalationController(alertService)
	silenceController := controllers.NewSilenceController(alertService)
	inspectionController := controllers.NewInspectionController(alertService)
	diagnosisRepo := repositories.NewDiagnosisRepository(db)
	adviceRepo := repositories.NewSQLAdviceRepository(db)
	aiTemp := 0.3
	if v := os.Getenv("DBOPS_AI_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 2 {
			aiTemp = f
		}
	}
	aiConfig := aiprovider.Config{
		BaseURL:     cfg.AIBaseURL,
		APIKey:      cfg.AIAPIKey,
		Model:       cfg.AIModel,
		MaxTokens:   defaultInt(cfg.AIMaxTokens, 2048),
		Temperature: aiTemp,
	}
	var aiProvider aiprovider.Provider = nil
	if cfg.AIBaseURL != "" && cfg.AIAPIKey != "" {
		aiProvider = aiprovider.NewOpenAIProvider(aiConfig)
	} else {
		logInstance.Info("AI provider not configured (set DBOPS_AI_BASE_URL and DBOPS_AI_API_KEY to enable)")
	}
	aiService := services.NewAIService(aiProvider, diagnosisRepo, adviceRepo, monitorService)
	aiController := controllers.NewAIController(aiService)
	faultTplRepo := repositories.NewFaultTemplateRepository(db)
	faultExecRepo := repositories.NewFaultExecutionRepository(db)
	faultService := services.NewFaultService(faultTplRepo, faultExecRepo)
	faultController := controllers.NewFaultController(faultService)
	drillRepo := repositories.NewDrillRepository(db)
	drillReportRepo := repositories.NewDrillReportRepository(db)
	drillService := services.NewDrillService(drillRepo, drillReportRepo, faultExecRepo)
	drillController := controllers.NewDrillController(drillService)

	approvalRepo := repositories.NewApprovalRequestRepository(db)
	approvalService := services.NewApprovalService(approvalRepo, auditRepo)
	approvalController := controllers.NewApprovalController(approvalService)

	// B8: 长任务进度查询.
	taskController := controllers.NewTaskController(taskRepo)
	auditController := controllers.NewAuditController(auditService)

	// SSE task stream (MessageBus → WSHub bridge).
	wsHub := services.NewWSHub()
	messageBus := services.NewMessageBus()
	wsController := controllers.NewWSController(wsHub, messageBus)

	// Plugin management API.
	pluginController := controllers.NewPluginController(pluginRegistry)

	dataMigrationController := controllers.NewDataMigrationController(cfg, db)

	maskingRepo := repositories.NewMaskingRepository(db)
	maskingService := services.NewMaskingService(maskingRepo)
	maskingController := controllers.NewMaskingController(maskingService)

	keyVersionRepo := repositories.NewKeyVersionRepository(db)
	keyRotationService := services.NewKeyRotationService(keyVersionRepo, auditService)
	keyRotationController := controllers.NewKeyRotationController(keyRotationService, cfg.EncryptionKey)

	licenseRepo := repositories.NewLicenseRepository(db)
	licenseService := services.NewLicenseService(licenseRepo, auditService, cfg.EncryptionKey)
	licenseController := controllers.NewLicenseController(licenseService)

	r := gin.New()
	r.Use(gin.Recovery())
	// P0-3: 限制 body 上限 10MB, 防止大文件上传 DoS.
	maxMem := int64(10 << 20)
	if v := os.Getenv("DBOPS_MAX_MULTIPART_MEMORY"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			maxMem = n
		}
	}
	r.MaxMultipartMemory = maxMem
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
		healthTimeout := 2 * time.Second
		if v := os.Getenv("DBOPS_HEALTH_CHECK_TIMEOUT"); v != "" {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				healthTimeout = d
			}
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthTimeout)
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
			auth.POST("/logout", authController.Logout)
		}

		protected := api.Group("")
		// P0-2: 鉴权不再根据存储后端区分, 一律走真实 JWT 校验.
		// (db=nil 时 authService 处于 standalone 模式, Login 接受任意凭据, 但 ValidateToken 仍要求合法签名.)
		protected.Use(authController.ValidateToken)
		{
			authProtected := protected.Group("/auth")
			{
				authProtected.POST("/change-password", authController.ChangePassword)
				authProtected.POST("/reset-all-passwords", middleware.RequirePermission("admin"), authController.ResetAllPasswords)
				authProtected.GET("/me", authController.Me)
			}

			users := protected.Group("/users")
			users.Use(middleware.RequirePermission("user:manage"))
			{
				users.GET("", userController.List)
				users.POST("", userController.Create)
				users.GET("/:id", userController.GetByID)
				users.PUT("/:id", userController.Update)
				users.DELETE("/:id", userController.Delete)
				users.POST("/:id/reset-password", userController.ResetPassword)
				users.POST("/:id/enable", userController.Enable)
				users.POST("/:id/disable", userController.Disable)
				users.PUT("/:id/roles", userController.UpdateRoles)
			}

			roles := protected.Group("/roles")
			roles.Use(middleware.RequirePermission("user:manage"))
			{
				roles.GET("", roleController.List)
				roles.POST("", roleController.Create)
				roles.PUT("/:id", roleController.Update)
				roles.DELETE("/:id", roleController.Delete)
			}

			instances := protected.Group("/instances")
			{
				instances.GET("", instanceController.List)
				instances.GET("/:id", instanceController.GetByID)
				// B2: 写操作 (POST/PUT/DELETE) 需要 admin 角色, 任何登录用户都不能直接调.
				instances.POST("", middleware.RequirePermission("admin"), instanceController.Create)
				instances.POST("/batch", middleware.RequirePermission("admin"), instanceController.BatchCreate)
				instances.POST("/admin/batch-password", middleware.RequirePermission("admin"), instanceController.BatchUpdatePassword)
				instances.PUT("/:id", middleware.RequirePermission("admin"), instanceController.Update)
				instances.DELETE("/:id", middleware.RequirePermission("admin"), instanceController.Delete)
				instances.POST("/:id/detect-version", middleware.RequirePermission("admin"), instanceController.DetectVersion)
				instances.POST("/:id/deploy", middleware.RequirePermission("admin"), instanceController.Deploy)
				instances.POST("/:id/health-check", middleware.RequirePermission("admin"), instanceController.HealthCheck)
				instances.POST("/:id/admin", middleware.RequirePermission("admin"), instanceController.AdminAction)
				instances.POST("/:id/admin-action", middleware.RequirePermission("admin"), instanceController.AdminAction)
				instances.POST("/:id/force-reset-password", middleware.RequirePermission("admin"), instanceController.ForceResetPassword)
				instances.POST("/:id/recover-cluster", middleware.RequirePermission("admin"), instanceController.RecoverCluster)
				instances.PUT("/:id/status", middleware.RequirePermission("admin"), instanceController.UpdateStatus)
				instances.GET("/:id/credentials", middleware.RequirePermission("admin"), instanceController.GetCredentials)
				instances.GET("/:id/replication-status", middleware.RequirePermission("admin"), instanceController.GetReplicationStatus)
			}

			hosts := protected.Group("/hosts")
			{
				hosts.GET("", hostController.List)
				hosts.POST("", middleware.RequirePermission("admin"), hostController.Create)
				hosts.POST("/batch", middleware.RequirePermission("admin"), hostController.BatchCreate)
				hosts.POST("/agent/batch", middleware.RequirePermission("admin"), hostController.BatchAgentAction)
				hosts.GET("/test/:task_id", hostController.GetTestResult)
				hosts.GET("/:id", hostController.GetByID)
				hosts.PUT("/:id", middleware.RequirePermission("admin"), hostController.Update)
				hosts.DELETE("/:id", middleware.RequirePermission("admin"), hostController.Delete)
				hosts.POST("/:id/agent", middleware.RequirePermission("admin"), hostController.AgentAction)
				hosts.POST("/:id/test", middleware.RequirePermission("admin"), hostController.TestConnection)
				hosts.POST("/:id/scan-instances", middleware.RequirePermission("admin"), hostController.ScanInstances)
				hosts.GET("/:id/scan-instances/:task_id", hostController.GetScanResult)
				hosts.POST("/:id/scan-instances/register", middleware.RequirePermission("admin"), hostController.RegisterScannedInstance)
				hosts.POST("/:id/scan-instances/register-batch", middleware.RequirePermission("admin"), hostController.RegisterScannedInstances)
			}

			envChecks := protected.Group("/env-checks")
			{
				envChecks.POST("", envCheckController.Execute)
				envChecks.GET("/:id", envCheckController.GetByID)
				envChecks.GET("/:id/export", envCheckController.Export)
			}

			backups := protected.Group("/backups")
			{
				// B2: 备份策略创建是 admin 操作.
				backups.GET("/policies", backupController.ListPolicies)
				backups.POST("/policies", middleware.RequirePermission("admin"), backupController.CreatePolicy)
				backups.PUT("/policies/:id", middleware.RequirePermission("admin"), backupController.UpdatePolicy)
				backups.DELETE("/policies/:id", middleware.RequirePermission("admin"), backupController.DeletePolicy)
				backups.POST("/scan", middleware.RequirePermission("admin"), backupController.ScanBackups)
				backups.POST("/restore", middleware.RequirePermission("admin"), backupController.RestoreBackup)
				backups.POST("", middleware.RequirePermission("admin"), backupController.ExecuteBackup)
				backups.GET("", backupController.ListBackups)
				backups.DELETE("/:id", middleware.RequirePermission("admin"), backupController.DeleteBackupRecord)
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
				paramTemplates.POST("/apply", middleware.RequirePermission("admin"), paramTemplateController.Apply)
			}

			deployments := protected.Group("/deployments")
			{
				// B2: 集群部署是高危操作, 必须 admin.
				deployments.POST("", middleware.RequirePermission("admin"), clusterDeployController.DeployCluster)
				deployments.POST("/validate", middleware.RequirePermission("admin"), clusterDeployController.ValidateClusterDeploy)
				deployments.POST("/precheck", middleware.RequirePermission("admin"), clusterDeployController.PreCheck)
				deployments.POST("/precheck/repair", middleware.RequirePermission("admin"), clusterDeployController.RepairPreCheck)
				deployments.POST("/mha", middleware.RequirePermission("admin"), clusterDeployController.DeployMHA)
				deployments.POST("/mgr", middleware.RequirePermission("admin"), clusterDeployController.DeployMGR)
				deployments.POST("/pxc", middleware.RequirePermission("admin"), clusterDeployController.DeployPXC)
				deployments.POST("/ha", middleware.RequirePermission("admin"), clusterDeployController.DeployHA)
				deployments.GET("", clusterDeployController.List)
				deployments.GET("/clusters", clusterDeployController.ListClusters)
				deployments.GET("/clusters/:cluster_id", clusterDeployController.GetClusterDetail)
				deployments.GET("/:id", clusterDeployController.GetDeploymentStatus)
				deployments.GET("/:id/plan", clusterDeployController.GetDeployPlan)
				deployments.POST("/:id/change-password", middleware.RequirePermission("admin"), clusterDeployController.ChangeClusterPassword)
				deployments.DELETE("/:id", middleware.RequirePermission("admin"), clusterDeployController.Destroy)
				deployments.POST("/:id/scale-out", middleware.RequirePermission("admin"), clusterDeployController.ScaleOut)
				deployments.POST("/:id/scale-in", middleware.RequirePermission("admin"), clusterDeployController.ScaleIn)
				deployments.POST("/:id/rebuild", middleware.RequirePermission("admin"), clusterDeployController.RebuildNode)
			}

			switches := protected.Group("/switch")
			{
				switches.POST("/single-to-mha", switchController.SingleToMHA)
				switches.POST("/single-to-mgr", switchController.SingleToMGR)
				switches.POST("/single-to-pxc", switchController.SingleToPXC)
				switches.POST("/cluster/role", switchController.SwitchRoleWithinCluster)
				switches.GET("/cluster/:cluster_id/role-history", switchController.ListRoleSwitchHistory)
			}

			topology := protected.Group("/topology")
			{
				topology.GET("/instances/:id", topologyController.GetInstanceTopology)
				topology.GET("/clusters/:cluster_id", topologyController.GetClusterTopology)
				topology.GET("/clusters/:cluster_id/graph", topologyController.GetTopologyGraph)
			}

			ha := protected.Group("/ha")
			{
				ha.GET("/health", healthCheckController.ExecuteHealthCheck)
				ha.GET("/health/failure-state", healthCheckController.GetFailureState)
				ha.GET("/health/detect", healthCheckController.DetectFailure)
				ha.POST("/health/batch", healthCheckController.BatchHealthCheck)
				ha.POST("/preflight", failoverController.PreflightFailover)
				// B2: 自动/手动 failover 是最危险操作, 强制 admin.
				ha.POST("/failover", middleware.RequirePermission("admin"), failoverController.ExecuteAutoFailover)
				ha.POST("/manual-switch", middleware.RequirePermission("admin"), failoverController.ExecuteManualFailover)
				ha.GET("/status", failoverController.GetClusterStatus)
			}

			upgrades := protected.Group("/upgrades")
			{
				upgrades.GET("", upgradeController.ListHistory)
				upgrades.POST("/plan", upgradeController.PlanUpgradePath)
				upgrades.POST("/check", upgradeController.CheckCompatibility)
				// B2: 实际触发升级 (in-place / logical / rolling / rollback) 全部 admin.
				upgrades.POST("/in-place", middleware.RequirePermission("admin"), upgradeController.ExecuteInPlaceUpgrade)
				upgrades.POST("/logical", middleware.RequirePermission("admin"), upgradeController.ExecuteLogicalMigration)
				upgrades.POST("/rolling", middleware.RequirePermission("admin"), upgradeController.ExecuteRollingUpgrade)
				upgrades.POST("/rollback", middleware.RequirePermission("admin"), upgradeController.RollbackUpgrade)
				upgrades.GET("/:id", upgradeController.GetUpgradeByID)
				upgrades.GET("/:id/report", upgradeController.GenerateUpgradeReport)
			}

			versions := protected.Group("/versions")
			{
				versions.GET("", versionController.List)
				versions.GET("/supported", versionController.ListSupported)
				versions.POST("/validate-path", versionController.ValidatePath)
				versions.GET("/:id", versionController.GetOne)
			}

			settings := protected.Group("/settings")
			{
				settings.GET("", settingsController.GetAll)
				settings.GET("/:key", settingsController.Get)
				settings.PUT("/:key", middleware.RequirePermission("admin"), settingsController.Set)
				settings.DELETE("/:key", middleware.RequirePermission("admin"), settingsController.Delete)
			}

			relay := protected.Group("/relay")
			{
				// Proxy HEAD request to test source connectivity (avoids browser CORS)
				relay.POST("/test-source", func(c *gin.Context) {
					var body struct {
						URL string `json:"url"`
					}
					if err := c.ShouldBindJSON(&body); err != nil || body.URL == "" {
						c.JSON(400, gin.H{"code": 400, "message": "url is required"})
						return
					}
					client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
						if len(via) >= 3 {
							return fmt.Errorf("too many redirects")
						}
						return nil
					}}
					method := "HEAD"
					// MySQL official returns 404 for HEAD, use GET instead
					if strings.Contains(body.URL, "dev.mysql.com") {
						method = "GET"
					}
					req, err := http.NewRequestWithContext(c.Request.Context(), method, body.URL, nil)
					if err != nil {
						c.JSON(200, gin.H{"code": 200, "data": gin.H{"ok": false, "error": err.Error()}})
						return
					}
					resp, err := client.Do(req)
					if err != nil {
						c.JSON(200, gin.H{"code": 200, "data": gin.H{"ok": false, "error": err.Error()}})
						return
					}
					resp.Body.Close()
					c.JSON(200, gin.H{"code": 200, "data": gin.H{"ok": resp.StatusCode >= 200 && resp.StatusCode < 400, "status": resp.StatusCode}})
				})
				relay.POST("/upload", func(c *gin.Context) {
					file, header, err := c.Request.FormFile("file")
					if err != nil {
						c.JSON(400, gin.H{"code": 400, "message": "file is required"})
						return
					}
					defer file.Close()
					subPath := c.PostForm("path")
					destDir := filepath.Join(cfg.DataDir, "packages")
					if subPath != "" {
						destDir = filepath.Join(destDir, subPath)
					}
					if err := os.MkdirAll(destDir, 0o755); err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "create dir failed: " + err.Error()})
						return
					}
					destPath := filepath.Join(destDir, header.Filename)
					out, err := os.Create(destPath)
					if err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "create file failed: " + err.Error()})
						return
					}
					defer out.Close()
					if _, err := io.Copy(out, file); err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "write file failed: " + err.Error()})
						return
					}
					c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"path": destPath, "size": header.Size}})
				})

				// Download a package from a mirror URL to the relay server directory
				relay.POST("/download-to-relay", func(c *gin.Context) {
					var body struct {
						URL        string `json:"url"`
						Filename   string `json:"filename"`
						TargetPath string `json:"target_path"`
					}
					if err := c.ShouldBindJSON(&body); err != nil || body.URL == "" || body.Filename == "" {
						c.JSON(400, gin.H{"code": 400, "message": "url and filename are required"})
						return
					}
					destDir := filepath.Join(cfg.DataDir, "packages")
					if body.TargetPath != "" {
						destDir = filepath.Join(destDir, body.TargetPath)
					}
					if err := os.MkdirAll(destDir, 0o755); err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "create dir failed: " + err.Error()})
						return
					}
					destPath := filepath.Join(destDir, body.Filename)
					// Check if already exists
					if _, err := os.Stat(destPath); err == nil {
						info, _ := os.Stat(destPath)
						c.JSON(200, gin.H{"code": 200, "message": "already exists", "data": gin.H{"path": destPath, "size": info.Size()}})
						return
					}
					// Download
					client := &http.Client{Timeout: 30 * time.Minute}
					req, err := http.NewRequestWithContext(c.Request.Context(), "GET", body.URL, nil)
					if err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "create request failed: " + err.Error()})
						return
					}
					resp, err := client.Do(req)
					if err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "download failed: " + err.Error()})
						return
					}
					defer resp.Body.Close()
					if resp.StatusCode != 200 {
						c.JSON(500, gin.H{"code": 500, "message": fmt.Sprintf("HTTP %d from %s", resp.StatusCode, body.URL)})
						return
					}
					out, err := os.Create(destPath)
					if err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "create file failed: " + err.Error()})
						return
					}
					written, err := io.Copy(out, resp.Body)
					out.Close()
					if err != nil {
						os.Remove(destPath)
						c.JSON(500, gin.H{"code": 500, "message": "write failed: " + err.Error()})
						return
					}
					c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"path": destPath, "size": written}})
				})

				// Scan local packages directory with multiple extensions
				relay.POST("/scan-remote", func(c *gin.Context) {
					var body struct {
						Path       string   `json:"path"`
						Extensions []string `json:"extensions"`
					}
					c.ShouldBindJSON(&body)
					scanDir := filepath.Join(cfg.DataDir, "packages")
					if body.Path != "" {
						scanDir = filepath.Join(scanDir, body.Path)
					}
					exts := body.Extensions
					if len(exts) == 0 {
						exts = []string{".tar.gz", ".tar.xz", ".tgz", ".tar.bz2", ".rpm", ".deb"}
					}
					packages := []gin.H{}
					filepath.Walk(scanDir, func(path string, info os.FileInfo, err error) error {
						if err != nil || info.IsDir() {
							return nil
						}
						name := strings.ToLower(info.Name())
						for _, ext := range exts {
							if strings.HasSuffix(name, ext) {
								rel, _ := filepath.Rel(scanDir, path)
								packages = append(packages, gin.H{"name": info.Name(), "size": fmt.Sprintf("%.1fMB", float64(info.Size())/(1024*1024)), "path": rel})
								break
							}
						}
						return nil
					})
					c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"packages": packages}})
				})

				// Scan a remote URL for packages (proxied to avoid CORS)
				relay.POST("/scan-url", func(c *gin.Context) {
					var body struct {
						URL        string   `json:"url"`
						Path       string   `json:"path"`
						Extensions []string `json:"extensions"`
					}
					if err := c.ShouldBindJSON(&body); err != nil || body.URL == "" {
						c.JSON(400, gin.H{"code": 400, "message": "url is required"})
						return
					}
					scanURL := strings.TrimRight(body.URL, "/")
					if body.Path != "" {
						scanURL += "/" + strings.TrimLeft(body.Path, "/")
					}
					exts := body.Extensions
					if len(exts) == 0 {
						exts = []string{".tar.gz", ".tar.xz", ".tgz", ".tar.bz2", ".rpm", ".deb"}
					}
					client := &http.Client{Timeout: 15 * time.Second}
					packages := []gin.H{}
					var scanDir func(url string, relPrefix string)
					scanDir = func(url string, relPrefix string) {
						req, err := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
						if err != nil {
							return
						}
						resp, err := client.Do(req)
						if err != nil || resp.StatusCode != 200 {
							if resp != nil {
								resp.Body.Close()
							}
							return
						}
						bodyBytes, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						html := string(bodyBytes)
						// Extract links
						linkRe := regexp.MustCompile(`href="([^"]+)"`)
						matches := linkRe.FindAllStringSubmatch(html, -1)
						dirs := []string{}
						for _, m := range matches {
							if len(m) < 2 {
								continue
							}
							href := m[1]
							if href == "../" || href == ".." || strings.HasPrefix(href, "?") {
								continue
							}
							if strings.HasSuffix(href, "/") {
								dirs = append(dirs, href)
								continue
							}
							lower := strings.ToLower(href)
							for _, ext := range exts {
								if strings.HasSuffix(lower, ext) {
									relPath := relPrefix + href
									packages = append(packages, gin.H{"name": href, "size": "-", "path": relPath})
									break
								}
							}
						}
						// Recurse into subdirectories (1 level)
						for _, dir := range dirs {
							dirName := strings.TrimSuffix(dir, "/")
							scanDir(url+"/"+dir, relPrefix+dirName+"/")
						}
					}
					scanDir(scanURL, "")
					c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"packages": packages}})
				})

				// Delete a package from relay server
				relay.POST("/delete-package", func(c *gin.Context) {
					var body struct {
						Path string `json:"path"`
					}
					if err := c.ShouldBindJSON(&body); err != nil || body.Path == "" {
						c.JSON(400, gin.H{"code": 400, "message": "path is required"})
						return
					}
					// Prevent path traversal
					clean := filepath.Clean(body.Path)
					if strings.Contains(clean, "..") {
						c.JSON(400, gin.H{"code": 400, "message": "invalid path"})
						return
					}
					fullPath := filepath.Join(cfg.DataDir, "packages", clean)
					if err := os.Remove(fullPath); err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "delete failed: " + err.Error()})
						return
					}
					c.JSON(200, gin.H{"code": 200, "message": "success"})
				})

				// Scan remote host packages via SSH (for relay servers)
				relay.POST("/scan-remote-ssh", func(c *gin.Context) {
					var body struct {
						HostID     string   `json:"host_id"`
						Path       string   `json:"path"`
						Extensions []string `json:"extensions"`
					}
					if err := c.ShouldBindJSON(&body); err != nil || body.HostID == "" {
						c.JSON(400, gin.H{"code": 400, "message": "host_id is required"})
						return
					}
					host, err := hostRepo.GetByID(c.Request.Context(), body.HostID)
					if err != nil {
						c.JSON(404, gin.H{"code": 404, "message": "host not found"})
						return
					}
					password, _ := utils.Decrypt(host.SSHCredential, cfg.EncryptionKey)
					if password == "" {
						c.JSON(400, gin.H{"code": 400, "message": "host has no SSH password configured"})
						return
					}
					sshClient, err := services.NewSSHClient(host.Address, host.SSHPort, host.SSHUser, password)
					if err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "SSH connect failed: " + err.Error()})
						return
					}
					defer sshClient.Close()
					scanPath := body.Path
					if scanPath == "" {
						scanPath = "/opt"
					}
					findCmd := fmt.Sprintf("find %s -maxdepth 3 -type f \\( -name '*.tar.gz' -o -name '*.tar.xz' -o -name '*.tgz' -o -name '*.tar.bz2' -o -name '*.rpm' -o -name '*.deb' -o -name '*.zip' \\) -printf '%%s\\t%%p\\n' 2>/dev/null", scanPath)
					out, _ := services.RunSSH(sshClient, findCmd)
					if strings.TrimSpace(out) == "" {
						simpleCmd := fmt.Sprintf("find %s -maxdepth 3 -type f \\( -name '*.tar.gz' -o -name '*.tar.xz' -o -name '*.tgz' -o -name '*.rpm' -o -name '*.deb' -o -name '*.zip' \\) -ls 2>/dev/null", scanPath)
						out, _ = services.RunSSH(sshClient, simpleCmd)
					}
					packages := []gin.H{}
					for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
						line = strings.TrimSpace(line)
						if line == "" {
							continue
						}
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							sizeStr := parts[0]
							filePath := parts[len(parts)-1]
							name := filepath.Base(filePath)
							var sizeBytes int64
							fmt.Sscanf(sizeStr, "%d", &sizeBytes)
							packages = append(packages, gin.H{"name": name, "size": fmt.Sprintf("%.1fMB", float64(sizeBytes)/(1024*1024)), "path": filePath})
						}
					}
					c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"packages": packages}})
				})

				// Download package from relay host to platform via SSH+SCP
				relay.POST("/pull-from-relay", func(c *gin.Context) {
					var body struct {
						HostID    string `json:"host_id"`
						FilePath  string `json:"file_path"`
						LocalPath string `json:"local_path"`
					}
					if err := c.ShouldBindJSON(&body); err != nil || body.HostID == "" || body.FilePath == "" {
						c.JSON(400, gin.H{"code": 400, "message": "host_id and file_path are required"})
						return
					}
					host, err := hostRepo.GetByID(c.Request.Context(), body.HostID)
					if err != nil {
						c.JSON(404, gin.H{"code": 404, "message": "host not found"})
						return
					}
					password, _ := utils.Decrypt(host.SSHCredential, cfg.EncryptionKey)
					if password == "" {
						c.JSON(400, gin.H{"code": 400, "message": "host has no SSH password configured"})
						return
					}
					sshClient, err := services.NewSSHClient(host.Address, host.SSHPort, host.SSHUser, password)
					if err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "SSH connect failed: " + err.Error()})
						return
					}
					defer sshClient.Close()
					localPath := body.LocalPath
					if localPath == "" {
						localPath = filepath.Join(cfg.DataDir, "packages", filepath.Base(body.FilePath))
					}
					if err := services.SCPDownload(sshClient, body.FilePath, localPath); err != nil {
						c.JSON(500, gin.H{"code": 500, "message": "SCP download failed: " + err.Error()})
						return
					}
					info, _ := os.Stat(localPath)
					size := int64(0)
					if info != nil {
						size = info.Size()
					}
					c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"path": localPath, "size": size}})
				})
			}

			migrations := protected.Group("/migrations")
			{
				migrations.GET("", migrationController.List)
				migrations.POST("/physical", migrationController.ExecutePhysical)
				migrations.POST("/replication", migrationController.ExecuteReplication)
				migrations.POST("/gtid", migrationController.ExecuteGTID)
				migrations.POST("/orchestrate", migrationController.Orchestrate)
				migrations.GET("/:id", migrationController.GetByID)
				migrations.GET("/:id/progress", migrationController.GetProgress)
				migrations.POST("/:id/verify", migrationController.Verify)
				migrations.POST("/:id/switch", migrationController.Switch)
				migrations.POST("/:id/cancel", migrationController.Cancel)
			}

			alerts := protected.Group("/alerts")
			{
				alerts.GET("/rules", alertController.ListAlertRules)
				// B2: 告警规则 / 渠道的写操作 admin 才能调.
				alerts.POST("/rules", middleware.RequirePermission("admin"), alertController.CreateAlertRule)
				alerts.GET("/rules/:id", alertController.GetAlertRule)
				alerts.PUT("/rules/:id", middleware.RequirePermission("admin"), alertController.UpdateAlertRule)
				alerts.DELETE("/rules/:id", middleware.RequirePermission("admin"), alertController.DeleteAlertRule)
				alerts.POST("/evaluate", alertController.EvaluateAlert)
				alerts.POST("/trigger", middleware.RequirePermission("admin"), alertController.TriggerAlert)
				alerts.GET("/history", alertController.GetAlertHistory)
				alerts.POST("/notifications", middleware.RequirePermission("admin"), alertController.SendNotification)
				alerts.GET("/notifications/channels", alertController.ListNotificationChannels)
				alerts.POST("/notifications/channels", middleware.RequirePermission("admin"), alertController.CreateNotificationChannel)
				alerts.GET("/notifications/channels/:id", alertController.GetNotificationChannel)
				alerts.PUT("/notifications/channels/:id", middleware.RequirePermission("admin"), alertController.UpdateNotificationChannel)
				alerts.DELETE("/notifications/channels/:id", middleware.RequirePermission("admin"), alertController.DeleteNotificationChannel)
				alerts.GET("/templates", alertTemplateController.List)
				alerts.POST("/templates", middleware.RequirePermission("admin"), alertTemplateController.Create)
				alerts.GET("/templates/:id", alertTemplateController.Get)
				alerts.DELETE("/templates/:id", middleware.RequirePermission("admin"), alertTemplateController.Delete)
				alerts.GET("/escalations", escalationController.List)
				alerts.POST("/escalations", middleware.RequirePermission("admin"), escalationController.Create)
				alerts.GET("/escalations/:id", escalationController.Get)
				alerts.DELETE("/escalations/:id", middleware.RequirePermission("admin"), escalationController.Delete)
				alerts.GET("/silences", silenceController.List)
				alerts.POST("/silences", middleware.RequirePermission("admin"), silenceController.Create)
				alerts.GET("/silences/:id", silenceController.Get)
				alerts.PUT("/silences/:id", middleware.RequirePermission("admin"), silenceController.Update)
				alerts.DELETE("/silences/:id", middleware.RequirePermission("admin"), silenceController.Delete)
				alerts.GET("/inspection/templates", inspectionController.ListTemplates)
				alerts.POST("/inspection/templates", middleware.RequirePermission("admin"), inspectionController.CreateTemplate)
				alerts.GET("/inspection/templates/:id", inspectionController.GetTemplate)
				alerts.PUT("/inspection/templates/:id", middleware.RequirePermission("admin"), inspectionController.UpdateTemplate)
				alerts.DELETE("/inspection/templates/:id", middleware.RequirePermission("admin"), inspectionController.DeleteTemplate)
				alerts.POST("/inspection/generate", middleware.RequirePermission("admin"), inspectionController.GenerateReport)
				alerts.GET("/inspection/reports", inspectionController.ListReports)
				alerts.GET("/inspection/reports/:id", inspectionController.GetReport)
				alerts.DELETE("/inspection/reports/:id", middleware.RequirePermission("admin"), inspectionController.DeleteReport)
			}

			faults := protected.Group("/faults")
			{
				faults.GET("/templates", faultController.ListTemplates)
				faults.POST("/templates", middleware.RequirePermission("admin"), faultController.CreateTemplate)
				faults.GET("/templates/:id", faultController.GetTemplate)
				faults.DELETE("/templates/:id", middleware.RequirePermission("admin"), faultController.DeleteTemplate)
				faults.POST("/execute", middleware.RequirePermission("admin"), faultController.Execute)
				faults.POST("/:id/rollback", middleware.RequirePermission("admin"), faultController.Rollback)
				faults.GET("/executions", faultController.ListExecutions)
				faults.GET("/executions/:id", faultController.GetExecution)
			}

			drills := protected.Group("/drills")
			{
				drills.GET("", drillController.List)
				drills.POST("", middleware.RequirePermission("admin"), drillController.Create)
				drills.GET("/:id", drillController.Get)
				drills.PUT("/:id", middleware.RequirePermission("admin"), drillController.Update)
				drills.DELETE("/:id", middleware.RequirePermission("admin"), drillController.Delete)
				drills.POST("/:id/start", middleware.RequirePermission("admin"), drillController.Start)
				drills.POST("/:id/complete", middleware.RequirePermission("admin"), drillController.Complete)
				drills.GET("/:id/report", drillController.GetReport)
			}

			ai := protected.Group("/ai")
			{
				ai.POST("/diagnosis", aiController.Diagnosis)
				ai.GET("/diagnoses", aiController.ListDiagnoses)
				ai.GET("/diagnoses/:id", aiController.GetDiagnosis)
				ai.POST("/sql-advisor", aiController.SQLAdvice)
				ai.GET("/sql-advices", aiController.ListSQLAdvice)
				ai.GET("/sql-advices/:id", aiController.GetSQLAdvice)
			}

			approvals := protected.Group("/approvals")
			{
				approvals.GET("", approvalController.ListApprovalRequests)
				approvals.POST("", middleware.RequirePermission("admin"), approvalController.CreateApprovalRequest)
				approvals.GET("/:id", approvalController.GetApprovalRequestByID)
				approvals.POST("/:id/approve", approvalController.ApproveRequest)
				approvals.POST("/:id/reject", approvalController.RejectRequest)
			}

			// B8: 通用 task 进度查询, 升级/备份/迁移共用.
			tasks := protected.Group("/tasks")
			{
				tasks.GET("/:id", taskController.GetByID)
				tasks.GET("", taskController.ListByInstance)
			}

			// SSE task progress stream.
			wsController.RegisterRoutes(protected)

			// Plugin management API.
			pluginRoutes := protected.Group("/plugins")
			{
				pluginRoutes.GET("", pluginController.List)
				pluginRoutes.GET("/:name", pluginController.Get)
			}

			auditLogs := protected.Group("/audit-logs")
			{
				auditLogs.GET("", auditController.ListAuditLogs)
				auditLogs.GET("/verify-chain", auditController.VerifyChain)
				auditLogs.GET("/:id", auditController.GetAuditLogByID)
			}

			dataMig := protected.Group("/data-migration")
			{
				dataMig.GET("/status", dataMigrationController.GetStatus)
				dataMig.POST("/import-legacy-json", dataMigrationController.ImportLegacyJSON)
				dataMig.POST("/migrate-to-mysql", dataMigrationController.MigrateToMySQL)
			}

			masking := protected.Group("/masking")
			{
				masking.GET("", maskingController.List)
				masking.POST("", middleware.RequirePermission("admin"), maskingController.Create)
				masking.GET("/:id", maskingController.GetByID)
				masking.PUT("/:id", middleware.RequirePermission("admin"), maskingController.Update)
				masking.DELETE("/:id", middleware.RequirePermission("admin"), maskingController.Delete)
			}

			keys := protected.Group("/keys")
			keys.Use(middleware.RequirePermission("admin"))
			{
				keys.POST("/rotate", keyRotationController.RotateKey)
				keys.GET("/versions", keyRotationController.ListKeyVersions)
			}

			licenseGroup := protected.Group("/license")
			{
				licenseGroup.GET("", licenseController.GetLicenseInfo)
				licenseGroup.GET("/features", licenseController.GetFeatures)
				licenseGroup.POST("/upload", middleware.RequirePermission("admin"), licenseController.UploadLicense)
			}
		}
	}

	if _, err := os.Stat("./frontend/dist"); err == nil {
		r.Static("/assets", "./frontend/dist/assets")
		r.NoRoute(func(c *gin.Context) {
			c.File("./frontend/dist/index.html")
		})
		logInstance.Info("Serving frontend SPA from ./frontend/dist")
	} else {
		logInstance.Info("Frontend SPA not found at ./frontend/dist, API-only mode")
	}

	logInstance.Info("Server starting on port " + cfg.ServerPort)
	readTimeout := time.Duration(cfg.ServerTimeouts.ReadTimeoutSec) * time.Second
	writeTimeout := time.Duration(cfg.ServerTimeouts.WriteTimeoutSec) * time.Second
	idleTimeout := time.Duration(cfg.ServerTimeouts.IdleTimeoutSec) * time.Second
	if readTimeout == 0 {
		readTimeout = 30 * time.Second
	}
	if writeTimeout == 0 {
		writeTimeout = 60 * time.Second
	}
	if idleTimeout == 0 {
		idleTimeout = 120 * time.Second
	}
	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	go func() {
		if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
			logInstance.Info("TLS enabled, certificate: " + cfg.TLSCertPath)
			if err := srv.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to start TLS server: %v", err)
			}
		} else {
			logInstance.Info("TLS not configured, serving HTTP")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logInstance.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	logInstance.Info("Server exited")
}

func runMigrations(db *repositories.Database) error {
	return repositories.RunMigrations(context.Background(), db)
}

// validateSecrets P0-1 防护: 任何缺省 / 短 / 明显示例值都直接拒启动.
func validateSecrets(cfg *config.Config) error {
	if cfg.DatabaseURL == "" || strings.Contains(cfg.DatabaseURL, "${DBOPS_DB_URL}") {
		return fmt.Errorf("DBOPS_DB_URL env var must be set (got %q). Set it to a valid DSN, e.g. export DBOPS_DB_URL='root:s3cret@tcp(host:3306)/dbops?parseTime=true&loc=Local'", cfg.DatabaseURL)
	}
	if len(cfg.JWTSecret) < 32 {
		return fmt.Errorf("jwt_secret must be set and >= 32 chars (current len=%d). Set DBOPS_JWT_SECRET or generate with: openssl rand -hex 32", len(cfg.JWTSecret))
	}
	if strings.Contains(cfg.JWTSecret, "PLEASE-CHANGE") || strings.Contains(cfg.JWTSecret, "INJECT_VIA_") {
		return fmt.Errorf("jwt_secret is a placeholder (%q); set DBOPS_JWT_SECRET to a strong random value", cfg.JWTSecret)
	}
	if len(cfg.EncryptionKey) < 32 {
		return fmt.Errorf("encryption_key must be set and >= 32 chars (current len=%d). Set DBOPS_ENCRYPTION_KEY or generate with: openssl rand -hex 32", len(cfg.EncryptionKey))
	}
	if strings.Contains(cfg.EncryptionKey, "PLEASE-CHANGE") || strings.Contains(cfg.EncryptionKey, "INJECT_VIA_") {
		return fmt.Errorf("encryption_key is a placeholder (%q); set DBOPS_ENCRYPTION_KEY to a strong random value", cfg.EncryptionKey)
	}
	if cfg.AgentToken == "" || len(cfg.AgentToken) < 16 {
		return fmt.Errorf("agent_token must be set and >= 16 chars. Set DBOPS_AGENT_TOKEN or generate with: openssl rand -hex 16")
	}
	if strings.Contains(cfg.AgentToken, "PLEASE-CHANGE") || strings.Contains(cfg.AgentToken, "INJECT_VIA_") {
		return fmt.Errorf("agent_token is a placeholder (%q); set DBOPS_AGENT_TOKEN to a strong random value", cfg.AgentToken)
	}
	return nil
}

func defaultInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func seedAuthSettings(ctx context.Context, db *repositories.Database, logInstance *zap.Logger) {
	if db == nil || db.Pool == nil {
		return
	}
	defaults := map[string]string{
		"auth.local_enabled":  "true",
		"auth.ldap_enabled":   "false",
		"auth.oauth2_enabled": "false",
		"auth.default_role":   "operator",
	}
	for key, value := range defaults {
		var existing string
		err := db.Pool.QueryRowContext(ctx, "SELECT value FROM platform_settings WHERE key = ?", key).Scan(&existing)
		if err == nil {
			continue
		}
		if _, err := db.Pool.ExecContext(ctx, "INSERT INTO platform_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)", key, value); err != nil {
			logInstance.Warn("Failed to seed auth setting " + key + ": " + err.Error())
		}
	}
}
