package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Field = zapcore.Field

var (
	Int    = zap.Int
	Int64  = zap.Int64
	String = zap.String
	Error  = zap.Error
	Any    = zap.Any
)
