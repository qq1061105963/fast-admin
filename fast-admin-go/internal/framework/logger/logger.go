// Package logger 封装 zap，提供全局可用的结构化日志实例，
// 对应 Java 侧 application-logging.yml 的日志能力。
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/config"
)

var global *zap.Logger = zap.NewNop()

// Init 根据配置初始化全局 logger，同时输出到控制台和文件（如配置了 Path）。
func Init(cfg config.LogConfig) error {
	level := zapcore.InfoLevel
	if cfg.Level != "" {
		if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
			return err
		}
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "time"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderCfg)

	writers := []zapcore.WriteSyncer{zapcore.AddSync(os.Stdout)}
	if cfg.Path != "" {
		f, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		writers = append(writers, zapcore.AddSync(f))
	}

	core := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(writers...), level)
	global = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return nil
}

// L 返回全局 logger；Init 调用前默认是 no-op logger，避免 nil panic。
func L() *zap.Logger {
	return global
}

func Sync() {
	_ = global.Sync()
}
