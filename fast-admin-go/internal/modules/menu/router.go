package menu

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/system/menu")
	g.GET("/userMenu", h.UserMenu)
	g.GET("/nameExists", h.NameExists)
	g.GET("/pathExists", h.PathExists)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "菜单管理", oplog.BizDelete), h.Delete)
	g.POST("", middleware.OperationLog(opWriter, "菜单管理", oplog.BizCreate), h.Create)
	g.PUT("", middleware.OperationLog(opWriter, "菜单管理", oplog.BizUpdate), h.Update)
	g.GET("", h.AllMenu)
}
