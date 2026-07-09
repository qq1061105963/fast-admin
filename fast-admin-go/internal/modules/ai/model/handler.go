package model

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

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
		Name:     c.Query("name"),
		Provider: c.Query("provider"),
		Enabled:  parseBoolPtr(c.Query("enabled")),
		Active:   parseBoolPtr(c.Query("active")),
		Page:     page, Size: size,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) Detail(c *gin.Context) {
	d, err := h.svc.Detail(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, d)
}

func (h *Handler) bind(c *gin.Context) (*SaveDTO, bool) {
	var dto SaveDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return nil, false
	}
	return &dto, true
}

func (h *Handler) Add(c *gin.Context) {
	dto, ok := h.bind(c)
	if !ok {
		return
	}
	if err := h.svc.Add(c.Request.Context(), dto); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) Update(c *gin.Context) {
	dto, ok := h.bind(c)
	if !ok {
		return
	}
	if err := h.svc.Update(c.Request.Context(), dto); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) FetchModels(c *gin.Context) {
	dto, ok := h.bind(c)
	if !ok {
		return
	}
	models, err := h.svc.FetchModels(c.Request.Context(), dto)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, models)
}

func (h *Handler) Test(c *gin.Context) {
	dto, ok := h.bind(c)
	if !ok {
		return
	}
	result, err := h.svc.Test(c.Request.Context(), dto)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) Activate(c *gin.Context) {
	if err := h.svc.Activate(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) ChangeEnabled(c *gin.Context) {
	enabled, _ := strconv.ParseBool(c.Query("enabled"))
	if err := h.svc.ChangeEnabled(c.Request.Context(), c.Param("id"), enabled); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) Del(c *gin.Context) {
	if err := h.svc.Del(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/ai/model")
	g.GET("", h.Page)
	g.GET("/:id", h.Detail)
	g.POST("", middleware.OperationLog(opWriter, "AI 模型配置", oplog.BizCreate), h.Add)
	g.PUT("", middleware.OperationLog(opWriter, "AI 模型配置", oplog.BizUpdate), h.Update)
	g.POST("/fetch-models", h.FetchModels)
	g.POST("/test", middleware.OperationLog(opWriter, "AI 模型配置", oplog.BizOther), h.Test)
	g.POST("/:id/activate", middleware.OperationLog(opWriter, "AI 模型配置", oplog.BizUpdate), h.Activate)
	g.POST("/:id/enabled", middleware.OperationLog(opWriter, "AI 模型配置", oplog.BizUpdate), h.ChangeEnabled)
	g.DELETE("/:id", middleware.OperationLog(opWriter, "AI 模型配置", oplog.BizDelete), h.Del)
}
