package mcp

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
)

type Handler struct {
	svc     *Service
	manager *Manager
}

func NewHandler(svc *Service, manager *Manager) *Handler {
	return &Handler{svc: svc, manager: manager}
}

func parseBoolPtr(v string) *bool {
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil
	}
	return &b
}

func (h *Handler) Page(c *gin.Context) {
	page, size := 1, 10
	if v, err := strconv.Atoi(c.Query("page")); err == nil {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("pageSize")); err == nil {
		size = v
	}
	list, total, err := h.svc.Page(c.Request.Context(), Query{
		Name: c.Query("name"), Transport: c.Query("transport"), Enabled: parseBoolPtr(c.Query("enabled")),
		Page: page, Size: size,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	for i := range list {
		h.manager.ApplyStatus(&list[i])
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) Detail(c *gin.Context) {
	server, err := h.svc.GetByIDOrErr(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.manager.ApplyStatus(server)
	response.Success(c, server)
}

func (h *Handler) Inspect(c *gin.Context) {
	dto, err := h.manager.Inspect(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, dto)
}

func (h *Handler) Add(c *gin.Context) {
	var dto SaveDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	server, err := h.svc.Add(c.Request.Context(), &dto)
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.manager.ReloadOne(server.ID)
	response.Success(c, nil)
}

func (h *Handler) Update(c *gin.Context) {
	var dto SaveDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	if err := h.svc.Update(c.Request.Context(), &dto); err != nil {
		response.Fail(c, err)
		return
	}
	h.manager.ReloadOne(dto.ID)
	response.Success(c, nil)
}

func (h *Handler) Del(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Del(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	h.manager.Remove(id)
	response.Success(c, nil)
}

func (h *Handler) ChangeEnabled(c *gin.Context) {
	id := c.Param("id")
	enabled, _ := strconv.ParseBool(c.Query("enabled"))
	if err := h.svc.ChangeEnabled(c.Request.Context(), id, enabled); err != nil {
		response.Fail(c, err)
		return
	}
	h.manager.ReloadOne(id)
	response.Success(c, nil)
}

func (h *Handler) Reload(c *gin.Context) {
	h.manager.ReloadOne(c.Param("id"))
	response.Success(c, nil)
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/ai/mcp/server")
	g.GET("", h.Page)
	g.GET("/:id", h.Detail)
	g.GET("/:id/inspect", h.Inspect)
	g.POST("", middleware.OperationLog(opWriter, "MCP 服务配置", oplog.BizCreate), h.Add)
	g.PUT("", middleware.OperationLog(opWriter, "MCP 服务配置", oplog.BizUpdate), h.Update)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "MCP 服务配置", oplog.BizDelete), h.Del)
	g.POST("/:id/enabled", middleware.OperationLog(opWriter, "MCP 服务配置", oplog.BizUpdate), h.ChangeEnabled)
	g.POST("/:id/reload", middleware.OperationLog(opWriter, "MCP 服务配置", oplog.BizUpdate), h.Reload)
}
