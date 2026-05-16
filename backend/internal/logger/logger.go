package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func New(level, path string, maxSize, maxBackups, maxAge int) (*zap.Logger, error) {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "timestamp"
	encoder := zapcore.NewJSONEncoder(cfg)
	w := zapcore.AddSync(&lumberjack.Logger{Filename: path, MaxSize: maxSize, MaxBackups: maxBackups, MaxAge: maxAge})
	lvl := zap.InfoLevel
	if err := lvl.Set(level); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	core := zapcore.NewCore(encoder, w, lvl)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}
