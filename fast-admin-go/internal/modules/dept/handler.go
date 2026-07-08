package dept

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

func (h *Handler) Tree(c *gin.Context) {
	tree, err := h.svc.Tree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, tree)
}

func (h *Handler) All(c *gin.Context) {
	list, err := h.svc.ListEnabled(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

func (h *Handler) NameExists(c *gin.Context) {
	exists, err := h.svc.NameExists(c.Request.Context(), c.Query("id"), c.Query("pid"), c.Query("name"))
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
	d, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, d)
}

func (h *Handler) Update(c *gin.Context) {
	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	d, err := h.svc.Update(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, d)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
