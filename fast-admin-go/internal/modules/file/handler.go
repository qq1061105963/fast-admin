package file

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (h *Handler) Page(c *gin.Context) {
	page, size := 1, 10
	if v, err := strconv.Atoi(c.Query("page")); err == nil {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("pageSize")); err == nil {
		size = v
	}
	list, total, err := h.svc.Page(c.Request.Context(), Query{
		Name: c.Query("name"), StorageType: c.Query("storageType"),
		BizType: c.Query("bizType"), BizID: c.Query("bizId"), Ext: c.Query("ext"),
		Page: page, Size: size,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) Upload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, errs.ErrInternal.Wrap(err))
		return
	}
	defer f.Close()

	result, err := h.svc.Upload(c.Request.Context(), UploadInput{
		Reader: f, Filename: fh.Filename, Size: fh.Size, ContentType: fh.Header.Get("Content-Type"),
		BizType: c.PostForm("bizType"), BizID: c.PostForm("bizId"),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) Download(c *gin.Context) {
	rc, f, err := h.svc.Download(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer rc.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.QueryEscape(f.OriginalName)))
	c.Header("Content-Type", "application/octet-stream")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
