package role

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/system/role")
	g.GET("/select", h.Select)
	g.GET("/nameExists", h.NameExists)
	g.GET("/:id", h.Detail)
	g.POST("", middleware.OperationLog(opWriter, "角色管理", oplog.BizCreate), h.Create)
	g.PUT("", middleware.OperationLog(opWriter, "角色管理", oplog.BizUpdate), h.Update)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "角色管理", oplog.BizDelete), h.Delete)
	g.GET("", h.Page)
}
