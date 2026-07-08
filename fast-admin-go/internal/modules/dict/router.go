package dict

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	t := rg.Group("/system/dict/type")
	t.GET("/all", h.AllTypes)
	t.POST("", middleware.OperationLog(opWriter, "字典类型", oplog.BizCreate), h.CreateType)
	t.PUT("", middleware.OperationLog(opWriter, "字典类型", oplog.BizUpdate), h.UpdateType)
	t.DELETE("/:id", middleware.OperationLog(opWriter, "字典类型", oplog.BizDelete), h.DeleteType)
	t.GET("", h.PageType)

	d := rg.Group("/system/dict/data")
	d.GET("/type/:dictType", h.ListByType)
	d.POST("", middleware.OperationLog(opWriter, "字典数据", oplog.BizCreate), h.CreateData)
	d.PUT("", middleware.OperationLog(opWriter, "字典数据", oplog.BizUpdate), h.UpdateData)
	d.DELETE("/:id", middleware.OperationLog(opWriter, "字典数据", oplog.BizDelete), h.DeleteData)
	d.GET("", h.PageData)
}
