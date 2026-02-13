package logger

import (
	"go.uber.org/zap"
)


type ILogger interface {
	Info(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Warning(msg string, fields ...Field)
}

type logger struct {
	zap *zap.Logger
}

func (l logger) Info(msg string, fields ...Field) {
	l.zap.Info(msg, fields...)
}

func (l logger) Error(msg string, fields ...Field) {
	l.zap.Error(msg, fields...)
}

func (l logger) Warning(msg string, fields ...Field) {
	l.zap.Warn(msg, fields...)
}

func New(namespace string) ILogger {
	return logger{
		zap: newZapLogger(namespace),
	}
}

func newZapLogger(namespace string) *zap.Logger {
	cfg := zap.NewDevelopmentConfig() // yoki zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout"}
	cfg.InitialFields = map[string]interface{}{
		"namespace": namespace,
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger
}
