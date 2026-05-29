package main

import (
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/pkg/config"
	"github.com/monkeycode/mysql-ops-platform/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("Starting MySQL Ops Platform...")

	r := gin.Default()
	
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Info("Server starting on port " + cfg.ServerPort)
	r.Run(":" + cfg.ServerPort)
}