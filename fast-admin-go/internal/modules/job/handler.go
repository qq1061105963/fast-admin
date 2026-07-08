package job

import (
	"strconv"

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

func (h *Handler) Page(c *gin.Context) {
	page, size := 1, 10
	if v, err := strconv.Atoi(c.Query("page")); err == nil {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("pageSize")); err == nil {
		size = v
	}
	list, total, err := h.svc.Page(c.Request.Context(), Query{
		JobName: c.Query("jobName"), JobGroup: c.Query("jobGroup"),
		Status: statusPtr(c.Query("status")), Page: page, Size: size,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) Detail(c *gin.Context) {
	j, err := h.svc.Detail(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, j)
}

func (h *Handler) Create(c *gin.Context) {
	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	j, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, j)
}

func (h *Handler) Update(c *gin.Context) {
	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	j, err := h.svc.Update(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, j)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) Start(c *gin.Context) {
	if err := h.svc.Start(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) Pause(c *gin.Context) {
	if err := h.svc.Pause(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) Run(c *gin.Context) {
	if err := h.svc.RunOnce(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) PageLog(c *gin.Context) {
	page, size := 1, 10
	if v, err := strconv.Atoi(c.Query("page")); err == nil {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("pageSize")); err == nil {
		size = v
	}
	list, total, err := h.svc.PageLog(c.Request.Context(), LogQuery{
		JobID: c.Query("jobId"), JobName: c.Query("jobName"),
		Status: statusPtr(c.Query("status")), Page: page, Size: size,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) DeleteLog(c *gin.Context) {
	if err := h.svc.DeleteLog(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) CleanLog(c *gin.Context) {
	if err := h.svc.CleanLog(c.Request.Context()); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
