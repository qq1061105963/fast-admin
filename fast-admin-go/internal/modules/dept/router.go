package dept

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/system/dept")
	g.GET("/nameExists", h.NameExists)
	g.GET("/all", h.All)
	g.POST("", middleware.OperationLog(opWriter, "部门管理", oplog.BizCreate), h.Create)
	g.PUT("", middleware.OperationLog(opWriter, "部门管理", oplog.BizUpdate), h.Update)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "部门管理", oplog.BizDelete), h.Delete)
	g.GET("", h.Tree)
}
