package rag

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/file"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func pageParams(c *gin.Context) (int, int) {
	page, size := 1, 10
	if v, err := strconv.Atoi(c.Query("page")); err == nil {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("pageSize")); err == nil {
		size = v
	}
	return page, size
}

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

// ---- 向量库 ----

func (h *Handler) VectorStoreStatus(c *gin.Context) {
	response.Success(c, h.svc.VectorStoreStatus(c.Request.Context()))
}

func (h *Handler) Collections(c *gin.Context) {
	list, err := h.svc.Collections(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

// ---- 知识库 ----

func (h *Handler) PageKB(c *gin.Context) {
	page, size := pageParams(c)
	list, total, err := h.svc.PageKB(c.Request.Context(), KBQuery{
		Name: c.Query("name"), Enabled: parseBoolPtr(c.Query("enabled")), Page: page, Size: size,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) DetailKB(c *gin.Context) {
	kb, err := h.svc.GetKBOrErr(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, kb)
}

func (h *Handler) AddKB(c *gin.Context) {
	var dto KBSaveDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	if err := h.svc.AddKB(c.Request.Context(), &dto); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) UpdateKB(c *gin.Context) {
	var dto KBSaveDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	if err := h.svc.UpdateKB(c.Request.Context(), &dto); err != nil {
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

func (h *Handler) DeleteKB(c *gin.Context) {
	if err := h.svc.DelKB(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// ---- 文档 ----

func (h *Handler) DocumentPage(c *gin.Context) {
	page, size := pageParams(c)
	list, total, err := h.svc.DocumentPage(c.Request.Context(), DocQuery{
		KnowledgeBaseID: c.Query("knowledgeBaseId"), Status: c.Query("status"), Page: page, Size: size,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) DocumentDetail(c *gin.Context) {
	doc, err := h.svc.GetDocumentDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, doc)
}

func (h *Handler) ChunkPage(c *gin.Context) {
	page, size := pageParams(c)
	list, total, err := h.svc.ChunkPage(c.Request.Context(), c.Param("id"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) Upload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.New(40122, 400, "请选择要上传的文件"))
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	defer src.Close()
	doc, err := h.svc.UploadDocument(c.Request.Context(), c.Param("id"), file.UploadInput{
		Reader:      src,
		Filename:    fileHeader.Filename,
		Size:        fileHeader.Size,
		ContentType: fileHeader.Header.Get("Content-Type"),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, doc)
}

func (h *Handler) Reindex(c *gin.Context) {
	if err := h.svc.ReindexDocument(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	deleteSourceFile, _ := strconv.ParseBool(c.Query("deleteSourceFile"))
	if err := h.svc.DeleteDocument(c.Request.Context(), c.Param("id"), deleteSourceFile); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// ---- 召回测试 ----

type recallRequest struct {
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	Query           string `json:"query"`
	TopK            *int   `json:"topK"`
}

func (h *Handler) RecallTest(c *gin.Context) {
	var req recallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	result, err := h.svc.Recall(c.Request.Context(), req.KnowledgeBaseID, req.Query, req.TopK)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/ai/rag")
	g.GET("/vector-store/status", h.VectorStoreStatus)
	g.GET("/vector-store/collections", h.Collections)

	g.GET("/knowledge", h.PageKB)
	g.GET("/knowledge/:id", h.DetailKB)
	g.POST("/knowledge", middleware.OperationLog(opWriter, "AI 知识库", oplog.BizCreate), h.AddKB)
	g.PUT("/knowledge", middleware.OperationLog(opWriter, "AI 知识库", oplog.BizUpdate), h.UpdateKB)
	g.POST("/knowledge/:id/enabled", middleware.OperationLog(opWriter, "AI 知识库", oplog.BizUpdate), h.ChangeEnabled)
	g.DELETE("/knowledge/:id", middleware.OperationLog(opWriter, "AI 知识库", oplog.BizDelete), h.DeleteKB)

	g.GET("/documents", h.DocumentPage)
	g.GET("/documents/:id", h.DocumentDetail)
	g.GET("/documents/:id/chunks", h.ChunkPage)
	g.POST("/knowledge/:id/documents/upload", middleware.OperationLog(opWriter, "AI 知识库文档", oplog.BizCreate), h.Upload)
	g.POST("/documents/:id/reindex", middleware.OperationLog(opWriter, "AI 知识库文档", oplog.BizUpdate), h.Reindex)
	g.DELETE("/documents/:id", middleware.OperationLog(opWriter, "AI 知识库文档", oplog.BizDelete), h.DeleteDocument)

	g.POST("/recall-test", h.RecallTest)
}
