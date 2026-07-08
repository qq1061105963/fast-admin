package job

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/system/job")
	g.GET("/:id", h.Detail)
	g.POST("/:id/start", middleware.OperationLog(opWriter, "定时任务", oplog.BizUpdate), h.Start)
	g.POST("/:id/pause", middleware.OperationLog(opWriter, "定时任务", oplog.BizUpdate), h.Pause)
	g.POST("/:id/run", middleware.OperationLog(opWriter, "定时任务", oplog.BizOther), h.Run)
	g.POST("", middleware.OperationLog(opWriter, "定时任务", oplog.BizCreate), h.Create)
	g.PUT("", middleware.OperationLog(opWriter, "定时任务", oplog.BizUpdate), h.Update)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "定时任务", oplog.BizDelete), h.Delete)
	g.GET("", h.Page)

	logGroup := rg.Group("/system/job/log")
	logGroup.GET("", h.PageLog)
	logGroup.DELETE("/clean", h.CleanLog)
	logGroup.DELETE("/:id", h.DeleteLog)
}
