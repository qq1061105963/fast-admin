package syslog

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type pageParams struct {
	Page int `form:"page,default=1"`
	Size int `form:"pageSize,default=10"`
}

func statusPtr(s string) *int8 {
	if s == "" {
		return nil
	}
	var v int8
	if s == "1" {
		v = 1
	}
	return &v
}

func (h *Handler) PageOperation(c *gin.Context) {
	var p pageParams
	_ = c.ShouldBindQuery(&p)
	q := OperationLogQuery{
		Title: c.Query("title"), BusinessType: c.Query("businessType"), Username: c.Query("username"),
		Status: statusPtr(c.Query("status")), Page: p.Page, Size: p.Size,
	}
	list, total, err := h.svc.PageOperation(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) DetailOperation(c *gin.Context) {
	row, err := h.svc.DetailOperation(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, row)
}

func (h *Handler) DeleteOperation(c *gin.Context) {
	if err := h.svc.DeleteOperation(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) CleanOperation(c *gin.Context) {
	if err := h.svc.CleanOperation(c.Request.Context()); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) PageLogin(c *gin.Context) {
	var p pageParams
	_ = c.ShouldBindQuery(&p)
	q := LoginLogQuery{
		Username: c.Query("username"), IP: c.Query("ip"), Status: statusPtr(c.Query("status")),
		Type: c.Query("type"), Page: p.Page, Size: p.Size,
	}
	list, total, err := h.svc.PageLogin(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) DeleteLogin(c *gin.Context) {
	if err := h.svc.DeleteLogin(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) CleanLogin(c *gin.Context) {
	if err := h.svc.CleanLogin(c.Request.Context()); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
