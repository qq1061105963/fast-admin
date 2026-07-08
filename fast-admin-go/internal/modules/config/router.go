package config

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/system/config")
	g.GET("/:id", h.Detail)
	g.POST("", middleware.OperationLog(opWriter, "系统配置", oplog.BizCreate), h.Create)
	g.PUT("", middleware.OperationLog(opWriter, "系统配置", oplog.BizUpdate), h.Update)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "系统配置", oplog.BizDelete), h.Delete)
	g.GET("", h.Page)
}
