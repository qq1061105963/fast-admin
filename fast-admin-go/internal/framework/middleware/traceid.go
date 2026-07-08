package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
)

// TraceID 给每个请求生成一个链路追踪 ID，写入 gin.Context，供 response 包和日志使用，
// 对应 Java 侧从 MDC 里取的 traceId。
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Trace-Id")
		if id == "" {
			id = ksuid.New().String()
		}
		c.Set(response.TraceIDKey, id)
		c.Header("X-Trace-Id", id)
		c.Next()
	}
}
