package online

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

func (h *Handler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

func (h *Handler) Kickout(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		response.Fail(c, errs.ErrBadRequest)
		return
	}
	if err := h.svc.Kickout(c.Request.Context(), token); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
