package file

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/system/file")
	g.POST("/upload", middleware.OperationLog(opWriter, "文件管理", oplog.BizCreate), h.Upload)
	g.GET("/:id/download", h.Download)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "文件管理", oplog.BizDelete), h.Delete)
	g.GET("", h.Page)
}
