package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 是一个基础的跨域中间件，生产环境建议把 Access-Control-Allow-Origin
// 收敛为具体的前端域名列表，而不是这里的开发期通配写法。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
