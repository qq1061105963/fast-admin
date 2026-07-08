package syslog

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	op := rg.Group("/system/log/operation")
	op.GET("", h.PageOperation)
	op.GET("/:id", h.DetailOperation)
	op.DELETE("/clean", h.CleanOperation)
	op.DELETE("/:id", h.DeleteOperation)

	login := rg.Group("/system/log/login")
	login.GET("", h.PageLogin)
	login.DELETE("/clean", h.CleanLogin)
	login.DELETE("/:id", h.DeleteLogin)
}
