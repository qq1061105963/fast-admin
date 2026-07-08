package user

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/system/user")
	g.GET("/info", h.Info)
	g.PUT("/profile", middleware.OperationLog(opWriter, "个人资料", oplog.BizUpdate), h.UpdateProfile)
	g.PUT("/avatar", middleware.OperationLog(opWriter, "个人资料", oplog.BizUpdate), h.ChangeAvatar)
	g.PUT("/password", middleware.OperationLog(opWriter, "个人资料", oplog.BizUpdate), h.ChangePassword)
	g.POST("", middleware.OperationLog(opWriter, "用户管理", oplog.BizCreate), h.Create)
	g.PUT("", middleware.OperationLog(opWriter, "用户管理", oplog.BizUpdate), h.Update)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "用户管理", oplog.BizDelete), h.Delete)
	g.GET("", h.Page)
}
