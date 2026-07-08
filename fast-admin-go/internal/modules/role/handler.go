package role

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

func (h *Handler) Page(c *gin.Context) {
	var q PageQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	list, total, err := h.svc.Page(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) Select(c *gin.Context) {
	options, err := h.svc.Select(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, options)
}

func (h *Handler) Detail(c *gin.Context) {
	dto, err := h.svc.Detail(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, dto)
}

func (h *Handler) NameExists(c *gin.Context) {
	exists, err := h.svc.NameExists(c.Request.Context(), c.Query("id"), c.Query("name"))
	if err != nil {
		response.Fail(c, errs.ErrInternal.Wrap(err))
		return
	}
	response.Success(c, exists)
}

func (h *Handler) Create(c *gin.Context) {
	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	dto, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, dto)
}

func (h *Handler) Update(c *gin.Context) {
	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	dto, err := h.svc.Update(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, dto)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
