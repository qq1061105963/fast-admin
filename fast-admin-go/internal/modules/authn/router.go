package authn

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

// RegisterRoutes 需要区分公开路由和需要登录态的路由：/auth/login 本身就是登录入口，
// 不能挂在鉴权中间件之后；/auth/logout、/auth/codes 需要登录态。
func RegisterRoutes(public, protected *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	public.POST("/auth/login", middleware.OperationLog(opWriter, "登录", oplog.BizOther), h.Login)
	protected.POST("/auth/logout", middleware.OperationLog(opWriter, "登出", oplog.BizOther), h.Logout)
	protected.GET("/auth/codes", h.Codes)
}
