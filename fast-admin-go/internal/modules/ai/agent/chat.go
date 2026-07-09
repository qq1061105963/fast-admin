package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/mcp"
	aimodel "github.com/SirYuxuan/fast-admin-go/internal/modules/ai/model"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/rag"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/settings"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/tool"
)

// ChatRequest 对应 AiChatRequest。
type ChatRequest struct {
	SessionID        string   `json:"sessionId"`
	Message          string   `json:"message"`
	ToolMode         string   `json:"toolMode"`
	ToolCodes        []string `json:"toolCodes"`
	MCPMode          string   `json:"mcpMode"`
	MCPServerIDs     []string `json:"mcpServerIds"`
	RagMode          string   `json:"ragMode"`
	KnowledgeBaseIDs []string `json:"knowledgeBaseIds"`
}

// Service 是 AI 对话编排的核心，聚合模型、工具、MCP、知识库、历史、确认与审计。
type Service struct {
	set      *settings.Settings
	modelSvc *aimodel.Service
	toolSvc  *tool.Service
	toolExec *tool.Executor
	mcpMgr   *mcp.Manager
	ragSvc   *rag.Service
	history  *HistoryService
	confirm  *ConfirmationService
	audit    *AuditLogger
	stats    *StatsService
}

func NewService(set *settings.Settings, modelSvc *aimodel.Service, toolSvc *tool.Service, toolExec *tool.Executor,
	mcpMgr *mcp.Manager, ragSvc *rag.Service, history *HistoryService, confirm *ConfirmationService,
	audit *AuditLogger, stats *StatsService) *Service {
	return &Service{
		set: set, modelSvc: modelSvc, toolSvc: toolSvc, toolExec: toolExec, mcpMgr: mcpMgr, ragSvc: ragSvc,
		history: history, confirm: confirm, audit: audit, stats: stats,
	}
}

func (s *Service) History() *HistoryService      { return s.history }
func (s *Service) Confirm() *ConfirmationService { return s.confirm }
func (s *Service) Stats() *StatsService          { return s.stats }

// processItem 对应 Java 的 ProcessItem，序列化进 ai_chat_message.process_json。
type processItem struct {
	CostMs   *int64 `json:"costMs"`
	Detail   string `json:"detail,omitempty"`
	ID       string `json:"id"`
	Source   string `json:"source,omitempty"`
	Status   string `json:"status"`
	Title    string `json:"title,omitempty"`
	ToolName string `json:"toolName,omitempty"`
	Type     string `json:"type"`
}

type sseWriter struct {
	c  *gin.Context
	mu sync.Mutex
}

func (w *sseWriter) send(event SseEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, _ := json.Marshal(event)
	_, _ = w.c.Writer.Write([]byte("data: "))
	_, _ = w.c.Writer.Write(b)
	_, _ = w.c.Writer.Write([]byte("\n\n"))
	w.c.Writer.Flush()
}

// Chat 同步执行一次对话，把 SSE 帧写入 gin 响应流。请求 goroutine 直接阻塞到对话结束。
func (s *Service) Chat(c *gin.Context, req *ChatRequest, userID string, permissionCodes []string) {
	reqCtx := c.Request.Context()
	// 持久化用 WithoutCancel 派生的 ctx：客户端断开也要把消息落库，同时保留审计 actor。
	persistCtx := context.WithoutCancel(reqCtx)

	w := &sseWriter{c: c}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	var items []processItem
	recordThought := func(text string) {
		items = append(items, processItem{ID: uuid.NewString(), Status: "info", Title: firstNonEmpty(text, "正在思考"), Type: "thought"})
	}
	sendThought := func(text string) {
		recordThought(text)
		w.send(thoughtEvent(text))
	}

	// 当前模型信息
	activeModel, _ := s.modelSvc.GetActiveEnabled(persistCtx)
	var mName, mProvider, mCode string
	if activeModel != nil {
		mName, mProvider, mCode = activeModel.Name, activeModel.Provider, activeModel.Model
	}
	w.send(sessionEvent(sessionID, mName, mProvider, mCode))

	if !s.set.AssistantEnabled(reqCtx) {
		w.send(errorEvent("AI 助手已关闭"))
		return
	}
	if s.set.AssistantRequirePermission(reqCtx) && !containsStr(permissionCodes, "ai:assistant:use") {
		w.send(errorEvent("无权使用 AI 助手"))
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		w.send(errorEvent("请输入要发送的消息"))
		return
	}

	sendThought("正在分析问题，并判断是否需要调用工具或 MCP。")

	if activeModel == nil {
		w.send(errorEvent("未配置可用的 AI 模型，请在模型管理中启用一个模型"))
		return
	}
	if strings.TrimSpace(activeModel.APIKey) == "" {
		w.send(errorEvent("当前 AI 模型未配置 API Key"))
		return
	}
	cfg := modelConfig{
		Provider: activeModel.Provider, Model: activeModel.Model, APIKey: activeModel.APIKey,
		BaseURL: activeModel.BaseURL, Temperature: activeModel.Temperature, MaxTokens: activeModel.MaxTokens,
	}

	// 构建工具集：内置 + 配置工具 + MCP
	specs := s.buildSpecs(reqCtx, req, permissionCodes)
	specByName := map[string]tool.Spec{}
	var defs []toolDef
	builtinCount, mcpCount := 0, 0
	for _, sp := range specs {
		specByName[sp.Name] = sp
		defs = append(defs, toolDef{Name: sp.Name, Description: sp.Description, InputSchema: sp.InputSchema})
		if sp.Source == sourceMCP {
			mcpCount++
		} else {
			builtinCount++
		}
	}
	sendThought(fmt.Sprintf("已挂载工具：内置 %d 个，MCP %d 个", builtinCount, mcpCount))

	// 历史（仅此前轮次）→ 落库本轮用户消息
	s.history.EnsureSession(persistCtx, sessionID, userID, req.Message)
	history := s.history.LoadHistory(persistCtx, sessionID)
	s.history.SaveUserMessage(persistCtx, sessionID, req.Message)

	// RAG 检索
	systemPrompt := s.set.AssistantSystemPrompt(reqCtx)
	if ragCtx := s.ragSvc.RetrieveChatContext(reqCtx, req.RagMode, req.KnowledgeBaseIDs, req.Message); ragCtx != nil && strings.TrimSpace(ragCtx.Text) != "" {
		systemPrompt = systemPrompt + "\n\n" + ragCtx.Text
		sendThought("已从知识库召回 " + itoa(len(ragCtx.Sources)) + " 段参考资料：" + strings.Join(ragCtx.Sources, "、"))
	}

	sendThought("已加载上下文，正在生成回答。")

	// 工具执行器：确认 → 事件 → 调用 → 事件 → 审计 → 过程记录
	exec := s.toolExecutor(persistCtx, w, &items, specByName, sessionID, userID)

	maxIters := s.set.AssistantMaxToolIterations(reqCtx)
	emit := func(delta string) {
		if delta != "" {
			w.send(deltaEvent(delta))
		}
	}

	answer, usage, err := streamChat(reqCtx, cfg, systemPrompt, history, req.Message, defs, maxIters, emit, exec)
	if err != nil {
		logger.L().Sugar().Warnf("AI chat stream failed: %v", err)
		w.send(errorEvent("AI 响应失败：" + err.Error()))
		return
	}

	s.history.SaveAssistantMessage(persistCtx, sessionID, answer, marshalItems(items),
		mName, mProvider, mCode, usage.Prompt, usage.Completion, usage.Total)
	w.send(doneEvent(uuid.NewString()))
}

// toolExecutor 返回给 streamChat 的工具执行闭包。
func (s *Service) toolExecutor(ctx context.Context, w *sseWriter, items *[]processItem,
	specByName map[string]tool.Spec, sessionID, operatorID string) toolExec {
	return func(_ context.Context, name string, args map[string]any) (string, bool) {
		spec, ok := specByName[name]
		if !ok {
			return "工具不存在：" + name, true
		}
		argsJSON := toJSON(args)

		// execute_sql 需二次确认
		if spec.RequiresConfirmation {
			sql := ""
			if v, ok := args[spec.ConfirmArgKey]; ok {
				sql = fmt.Sprintf("%v", v)
			}
			confirmToken := uuid.NewString()
			w.send(toolPendingEvent(name, sql, confirmToken))
			if !s.confirm.WaitForConfirmation(confirmToken) {
				return "用户已取消 SQL 执行。", false
			}
		}

		recordToolStart(items, name, spec.Source, argsJSON)
		w.send(toolStartEvent(name, spec.Source, argsJSON))
		start := time.Now()
		result, err := spec.Call(ctx, args)
		cost := time.Since(start).Milliseconds()
		if err != nil {
			w.send(toolEndEvent(name, spec.Source, false, cost))
			recordToolEnd(items, name, spec.Source, false, cost)
			s.audit.Write(ctx, sessionID, operatorID, name, spec.Source, argsJSON, "", false, err.Error(), cost)
			return "工具执行失败：" + err.Error(), true
		}
		w.send(toolEndEvent(name, spec.Source, true, cost))
		recordToolEnd(items, name, spec.Source, true, cost)
		s.audit.Write(ctx, sessionID, operatorID, name, spec.Source, argsJSON, result, true, "", cost)
		return result, false
	}
}

func (s *Service) buildSpecs(ctx context.Context, req *ChatRequest, permissionCodes []string) []tool.Spec {
	toolMode := normalizeMode(req.ToolMode, "auto")
	specs := s.toolSvc.BuildSpecs(ctx, s.toolExec, permissionCodes, toolMode, req.ToolCodes)

	mcpMode := normalizeMode(req.MCPMode, "off")
	if mcpMode != "off" {
		var ids []string
		if mcpMode == "manual" {
			ids = req.MCPServerIDs
		}
		specs = append(specs, s.mcpMgr.ListSpecs(ctx, ids)...)
	}
	return specs
}

// ---- 过程记录 ----

func recordToolStart(items *[]processItem, toolName, source, argsJSON string) {
	title := toolTitle(source, toolName)
	*items = append(*items, processItem{
		CostMs: nil, Detail: argsJSON, ID: uuid.NewString(), Source: srcOr(source), Status: "running",
		Title: title, ToolName: toolName, Type: "tool",
	})
}

func recordToolEnd(items *[]processItem, toolName, source string, ok bool, cost int64) {
	status := "success"
	if !ok {
		status = "error"
	}
	c := cost
	src := srcOr(source)
	for i := len(*items) - 1; i >= 0; i-- {
		it := (*items)[i]
		if it.Type == "tool" && it.ToolName == toolName && it.Source == src && it.Status == "running" {
			(*items)[i].Status = status
			(*items)[i].CostMs = &c
			return
		}
	}
	*items = append(*items, processItem{
		CostMs: &c, ID: uuid.NewString(), Source: src, Status: status,
		Title: toolTitle(source, toolName), ToolName: toolName, Type: "tool",
	})
}

func toolTitle(source, toolName string) string {
	prefix := "工具"
	if source == sourceMCP {
		prefix = "MCP"
	}
	return prefix + "调用：" + toolName
}

func srcOr(source string) string {
	if source == "" {
		return sourceBuiltin
	}
	return source
}

func marshalItems(items []processItem) string {
	if len(items) == 0 {
		return ""
	}
	b, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(b)
}

// ---- 小工具 ----

func normalizeMode(mode, def string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "auto", "manual", "off":
		return m
	default:
		return def
	}
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func itoa(v int) string { return fmt.Sprintf("%d", v) }

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
