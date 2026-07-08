package online

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/system/online")
	g.GET("", h.List)
	g.DELETE("/:token", middleware.OperationLog(opWriter, "在线用户", oplog.BizForceLogout), h.Kickout)
}
