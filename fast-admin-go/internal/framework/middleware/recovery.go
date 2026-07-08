package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
)

// Recovery 捕获 handler 中的 panic，记录堆栈并返回统一的 500 响应，
// 避免一个未处理的 panic 打垮整个进程。
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		logger.L().Error("panic recovered",
			zap.Any("error", recovered),
			zap.String("path", c.Request.URL.Path),
		)
		response.Fail(c, errs.ErrInternal)
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
