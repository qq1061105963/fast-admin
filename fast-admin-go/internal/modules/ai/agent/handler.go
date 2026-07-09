package agent

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
)

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(timeLayout)
}

type Handler struct {
	svc  *Service
	repo *Repository
}

func NewHandler(svc *Service, repo *Repository) *Handler {
	return &Handler{svc: svc, repo: repo}
}

const timeLayout = "2006-01-02T15:04:05"

// ---- 对话 SSE ----

func (h *Handler) Chat(c *gin.Context) {
	session, ok := middleware.CurrentSession(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(200)
	c.Writer.Flush()

	h.svc.Chat(c, &req, session.UserID, session.Permissions)
}

// ---- 会话 ----

type sessionDTO struct {
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func (h *Handler) Sessions(c *gin.Context) {
	session, ok := middleware.CurrentSession(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	list, err := h.svc.History().ListSessions(c.Request.Context(), session.UserID)
	if err != nil {
		response.Fail(c, errs.ErrInternal.Wrap(err))
		return
	}
	out := make([]sessionDTO, 0, len(list))
	for _, s := range list {
		out = append(out, sessionDTO{
			SessionID: s.SessionID, Title: s.Title,
			CreatedAt: fmtTime(s.CreatedAt), UpdatedAt: fmtTime(s.UpdatedAt),
		})
	}
	response.Success(c, out)
}

type messageDTO struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ProcessJSON      string `json:"processJson"`
	ModelName        string `json:"modelName"`
	ModelProvider    string `json:"modelProvider"`
	ModelCode        string `json:"modelCode"`
	PromptTokens     *int   `json:"promptTokens"`
	CompletionTokens *int   `json:"completionTokens"`
	TotalTokens      *int   `json:"totalTokens"`
	CreatedAt        string `json:"createdAt"`
}

func (h *Handler) Messages(c *gin.Context) {
	session, ok := middleware.CurrentSession(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	list, err := h.svc.History().ListMessages(c.Request.Context(), c.Param("sessionId"), session.UserID)
	if err != nil {
		response.Fail(c, errs.ErrInternal.Wrap(err))
		return
	}
	out := make([]messageDTO, 0, len(list))
	for _, m := range list {
		out = append(out, messageDTO{
			Role: m.Role, Content: m.Content, ProcessJSON: m.ProcessJSON,
			ModelName: m.ModelName, ModelProvider: m.ModelProvider, ModelCode: m.ModelCode,
			PromptTokens: m.PromptTokens, CompletionTokens: m.CompletionTokens, TotalTokens: m.TotalTokens,
			CreatedAt: fmtTime(m.CreatedAt),
		})
	}
	response.Success(c, out)
}

func (h *Handler) DeleteSession(c *gin.Context) {
	session, ok := middleware.CurrentSession(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	if err := h.svc.History().DeleteSession(c.Request.Context(), c.Param("sessionId"), session.UserID); err != nil {
		response.Fail(c, errs.ErrInternal.Wrap(err))
		return
	}
	response.Success(c, nil)
}

func (h *Handler) Confirm(c *gin.Context) {
	confirmed := true
	if v := c.Query("confirmed"); v != "" {
		confirmed, _ = strconv.ParseBool(v)
	}
	if !h.svc.Confirm().Respond(c.Param("token"), confirmed) {
		response.Fail(c, errs.New(40160, 400, "确认令牌不存在或已超时"))
		return
	}
	response.Success(c, nil)
}

// ---- 用量统计 ----

func (h *Handler) UsageStats(c *gin.Context) {
	days := 14
	if v, err := strconv.Atoi(c.Query("days")); err == nil {
		days = v
	}
	stats, err := h.svc.Stats().Stats(c.Request.Context(), days)
	if err != nil {
		response.Fail(c, errs.ErrInternal.Wrap(err))
		return
	}
	response.Success(c, stats)
}

// ---- 工具调用日志 ----

func (h *Handler) ToolLogPage(c *gin.Context) {
	page, size := 1, 10
	if v, err := strconv.Atoi(c.Query("page")); err == nil {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("pageSize")); err == nil {
		size = v
	}
	var success *bool
	if v := c.Query("success"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			success = &b
		}
	}
	list, total, err := h.repo.PageToolLog(c.Request.Context(), ToolLogQuery{
		ToolName: c.Query("toolName"), Source: c.Query("source"), Success: success,
		SessionID: c.Query("sessionId"), OperatorID: c.Query("operatorId"), Page: page, Size: size,
	})
	if err != nil {
		response.Fail(c, errs.ErrInternal.Wrap(err))
		return
	}
	response.SuccessPage(c, list, total)
}

func (h *Handler) ToolLogDetail(c *gin.Context) {
	log, err := h.repo.GetToolLog(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, errs.New(40404, 404, "工具调用日志不存在"))
		return
	}
	response.Success(c, log)
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	g := rg.Group("/ai/agent")
	g.POST("/chat", h.Chat)
	g.GET("/sessions", h.Sessions)
	g.GET("/sessions/:sessionId/messages", h.Messages)
	g.DELETE("/sessions/:sessionId", h.DeleteSession)
	g.POST("/confirm/:token", h.Confirm)

	rg.GET("/ai/usage/stats", h.UsageStats)

	log := rg.Group("/ai/tool-log")
	log.GET("", h.ToolLogPage)
	log.GET("/:id", h.ToolLogDetail)
}
