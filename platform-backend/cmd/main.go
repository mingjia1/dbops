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
			logInstance.Warn("Migrations skipped", zap.String("error", err.Error()))
		}
	}

	userRepo := repositories.NewUserRepository(db)
	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	authController := controllers.NewAuthController(authService)

	instanceRepo := repositories.NewInstanceRepository(db)
	instanceService := services.NewInstanceService(instanceRepo)
	instanceController := controllers.NewInstanceController(instanceService)

	envCheckService := services.NewEnvironmentCheckService()
	envCheckController := controllers.NewEnvironmentCheckController(envCheckService)

	backupService := services.NewBackupService()
	backupController := controllers.NewBackupController(backupService)

	monitorService := services.NewMonitorService()
	monitorController := controllers.NewMonitorController(monitorService)

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
		}
	}

	logInstance.Info("Server starting on port " + cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func runMigrations(db *repositories.Database) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	return repositories.RunMigrations(ctx, conn.Conn())
}