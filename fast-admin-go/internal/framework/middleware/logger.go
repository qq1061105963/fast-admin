package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
)

// RequestLogger 记录每个请求的方法、路径、状态码、耗时，对应操作日志里的
// 基础访问日志部分（业务操作日志由 audit 中间件单独处理）。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logger.L().Info("http_request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
