package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(level string) *zap.Logger {
	var config zap.Config
	config = zap.NewProductionConfig()
	
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	
	config.Encoding = "json"
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}
	
	logger, err := config.Build()
	if err != nil {
		fallback, _ := zap.NewProduction()
		if fallback == nil {
			fallback = zap.NewNop()
		}
		return fallback
	}
	
	return logger
}