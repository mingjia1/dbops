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
	logInstance.Info("Starting MySQL Ops Platform...")

	db, err := repositories.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := runMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	userRepo := repositories.NewUserRepository(db)
	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	authController := controllers.NewAuthController(authService)

	instanceRepo := repositories.NewInstanceRepository(db)
	instanceService := services.NewInstanceService(instanceRepo)
	instanceController := controllers.NewInstanceController(instanceService)

	r := gin.Default()
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logInstance))
	r.Use(middleware.ErrorHandler())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", authController.Login)
			auth.POST("/register", authController.Register)
		}

		instances := api.Group("/instances")
		instances.Use(authController.ValidateToken)
		{
			instances.GET("", instanceController.List)
			instances.POST("", instanceController.Create)
			instances.GET("/:id", instanceController.GetByID)
			instances.PUT("/:id", instanceController.Update)
			instances.DELETE("/:id", instanceController.Delete)
			instances.POST("/:id/detect-version", instanceController.DetectVersion)
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