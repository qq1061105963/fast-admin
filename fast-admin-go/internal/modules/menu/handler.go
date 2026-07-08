package menu

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) UserMenu(c *gin.Context) {
	session, ok := middleware.CurrentSession(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	tree, err := h.svc.UserMenuTree(c.Request.Context(), session.UserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, tree)
}

func (h *Handler) AllMenu(c *gin.Context) {
	tree, err := h.svc.AllMenuTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, tree)
}

func (h *Handler) NameExists(c *gin.Context) {
	id := c.Query("id")
	name := c.Query("name")
	exists, err := h.svc.NameExists(c.Request.Context(), id, name)
	if err != nil {
		response.Fail(c, errs.ErrInternal.Wrap(err))
		return
	}
	response.Success(c, exists)
}

func (h *Handler) PathExists(c *gin.Context) {
	id := c.Query("id")
	path := c.Query("path")
	exists, err := h.svc.PathExists(c.Request.Context(), id, path)
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
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, m)
}

func (h *Handler) Update(c *gin.Context) {
	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	m, err := h.svc.Update(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, m)
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
