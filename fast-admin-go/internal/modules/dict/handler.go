package dict

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
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

func (h *Handler) PageType(c *gin.Context) {
	var p pageParams
	_ = c.ShouldBindQuery(&p)
	q := TypeQuery{
		DictName: c.Query("dictName"), DictType: c.Query("dictType"),
		Status: statusPtr(c.Query("status")), Page: p.Page, Size: p.Size,
	}
	list, total, err := h.svc.PageType(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) AllTypes(c *gin.Context) {
	list, err := h.svc.ListEnabledTypes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

func (h *Handler) CreateType(c *gin.Context) {
	var req TypeSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	t, err := h.svc.CreateType(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, t)
}

func (h *Handler) UpdateType(c *gin.Context) {
	var req TypeSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	t, err := h.svc.UpdateType(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, t)
}

func (h *Handler) DeleteType(c *gin.Context) {
	if err := h.svc.DeleteType(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) PageData(c *gin.Context) {
	var p pageParams
	_ = c.ShouldBindQuery(&p)
	q := DataQuery{
		DictType: c.Query("dictType"), DictLabel: c.Query("dictLabel"),
		Status: statusPtr(c.Query("status")), Page: p.Page, Size: p.Size,
	}
	list, total, err := h.svc.PageData(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) ListByType(c *gin.Context) {
	list, err := h.svc.ListByType(c.Request.Context(), c.Param("dictType"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

func (h *Handler) CreateData(c *gin.Context) {
	var req DataSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	d, err := h.svc.CreateData(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, d)
}

func (h *Handler) UpdateData(c *gin.Context) {
	var req DataSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	d, err := h.svc.UpdateData(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, d)
}

func (h *Handler) DeleteData(c *gin.Context) {
	if err := h.svc.DeleteData(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
