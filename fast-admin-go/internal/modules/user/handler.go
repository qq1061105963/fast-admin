package user

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

func currentUserID(c *gin.Context) (string, bool) {
	session, ok := middleware.CurrentSession(c)
	if !ok {
		return "", false
	}
	return session.UserID, true
}

func (h *Handler) Page(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	var q PageQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	list, total, err := h.svc.Page(c.Request.Context(), userID, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) Info(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	dto, err := h.svc.Info(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, dto)
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

func (h *Handler) ChangePassword(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	var req PasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	var req ProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	dto, err := h.svc.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, dto)
}

func (h *Handler) ChangeAvatar(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	if err := h.svc.ChangeAvatar(c.Request.Context(), userID, c.Query("avatar")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
