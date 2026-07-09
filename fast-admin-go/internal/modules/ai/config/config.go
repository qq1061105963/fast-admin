// Package config 实现 /ai/config 聚合配置接口，对应 Java 侧的 AiConfigController +
// AiConfigService + AiRagConfigService.current/save。所有值都落在 sys_config 表，读写
// 走 settings 包；密钥字段（Qdrant / Embedding API Key）读取时脱敏成 "******"，保存时
// 若仍是掩码则保留原值。
package config

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/settings"
)

const mask = "******"

// MCPReloader 抽象 MCP 客户端管理器的重载能力，在 ai.mcp.client.enabled 开关变化时触发，
// 由 mcp 模块实现并在 bootstrap 里回填，避免 config 直接依赖 mcp 包。
type MCPReloader interface {
	Reload()
}

// RagDTO 对应 AiRagConfigDto。密钥字段读出为掩码，保存时按掩码判定是否保留原值。
type RagDTO struct {
	Enabled            *bool  `json:"enabled"`
	CollectionName     string `json:"collectionName"`
	QdrantURL          string `json:"qdrantUrl"`
	QdrantAPIKey       string `json:"qdrantApiKey"`
	QdrantTimeoutMs    *int   `json:"qdrantTimeoutMs"`
	EmbeddingBaseURL   string `json:"embeddingBaseUrl"`
	EmbeddingAPIKey    string `json:"embeddingApiKey"`
	EmbeddingModel     string `json:"embeddingModel"`
	EmbeddingTimeoutMs *int   `json:"embeddingTimeoutMs"`
}

// DTO 对应 AiConfigDto。指针字段用于区分“未传”与“显式 false/0”，对齐 Java 的 Boolean/Integer 语义。
type DTO struct {
	AssistantEnabled           *bool   `json:"assistantEnabled"`
	AssistantRequirePermission *bool   `json:"assistantRequirePermission"`
	AssistantMaxToolIterations *int    `json:"assistantMaxToolIterations"`
	AssistantSystemPrompt      string  `json:"assistantSystemPrompt"`
	MCPClientEnabled           *bool   `json:"mcpClientEnabled"`
	ChatHistoryWindow          *int    `json:"chatHistoryWindow"`
	ReadonlySQLEnabled         *bool   `json:"readonlySqlEnabled"`
	ReadonlySQLPermissionCode  string  `json:"readonlySqlPermissionCode"`
	ReadonlySQLMaxRows         *int    `json:"readonlySqlMaxRows"`
	ExecuteSQLEnabled          *bool   `json:"executeSqlEnabled"`
	ExecuteSQLPermissionCode   string  `json:"executeSqlPermissionCode"`
	ExecuteSQLMaxRows          *int    `json:"executeSqlMaxRows"`
	SchemaToolEnabled          *bool   `json:"schemaToolEnabled"`
	SchemaToolPermissionCode   string  `json:"schemaToolPermissionCode"`
	Rag                        *RagDTO `json:"rag"`
}

// Service 聚合配置读写。mcp 字段可为 nil（尚未回填时静默跳过重载）。
type Service struct {
	set *settings.Settings
	mcp MCPReloader
}

func NewService(set *settings.Settings) *Service {
	return &Service{set: set}
}

// SetMCPReloader 在 bootstrap 里把 mcp 客户端管理器回填进来。
func (s *Service) SetMCPReloader(r MCPReloader) { s.mcp = r }

// Current 读取当前完整配置，密钥脱敏。
func (s *Service) Current(ctx context.Context) *DTO {
	set := s.set
	b := func(v bool) *bool { return &v }
	i := func(v int) *int { return &v }
	return &DTO{
		AssistantEnabled:           b(set.AssistantEnabled(ctx)),
		AssistantRequirePermission: b(set.AssistantRequirePermission(ctx)),
		AssistantMaxToolIterations: i(set.AssistantMaxToolIterations(ctx)),
		AssistantSystemPrompt:      set.AssistantSystemPrompt(ctx),
		MCPClientEnabled:           b(set.MCPClientEnabled(ctx)),
		ChatHistoryWindow:          i(set.ChatHistoryWindow(ctx)),
		ReadonlySQLEnabled:         b(set.ReadonlySQLEnabled(ctx)),
		ReadonlySQLPermissionCode:  set.ReadonlySQLPermissionCode(ctx),
		ReadonlySQLMaxRows:         i(set.ReadonlySQLMaxRows(ctx)),
		ExecuteSQLEnabled:          b(set.ExecuteSQLEnabled(ctx)),
		ExecuteSQLPermissionCode:   set.ExecuteSQLPermissionCode(ctx),
		ExecuteSQLMaxRows:          i(set.ExecuteSQLMaxRows(ctx)),
		SchemaToolEnabled:          b(set.SchemaToolEnabled(ctx)),
		SchemaToolPermissionCode:   set.SchemaToolPermissionCode(ctx),
		Rag: &RagDTO{
			Enabled:            b(set.RagEnabled(ctx)),
			CollectionName:     set.RagCollectionName(ctx),
			QdrantURL:          set.RagQdrantURL(ctx),
			QdrantAPIKey:       maskIfPresent(set.RagQdrantAPIKey(ctx)),
			QdrantTimeoutMs:    i(set.RagQdrantTimeoutMs(ctx)),
			EmbeddingBaseURL:   set.RagEmbeddingBaseURL(ctx),
			EmbeddingAPIKey:    maskIfPresent(set.RagEmbeddingAPIKey(ctx)),
			EmbeddingModel:     set.RagEmbeddingModel(ctx),
			EmbeddingTimeoutMs: i(set.RagEmbeddingTimeoutMs(ctx)),
		},
	}
}

// Save 写回全部配置。rag 块做必填校验；mcp 客户端开关变化时触发重载。
func (s *Service) Save(ctx context.Context, dto *DTO) error {
	if dto == nil {
		return errs.New(40010, 400, "AI 配置不能为空")
	}
	store := s.set.Store()
	oldMCP := s.set.MCPClientEnabled(ctx)

	up := func(key, name, value, remark string) error { return store.Upsert(ctx, key, name, value, remark) }

	_ = up(settings.AssistantEnabled, "AI助手开关", boolStr(notFalse(dto.AssistantEnabled)), "控制 AI 运维助手是否启用")
	_ = up(settings.AssistantRequirePermission, "AI助手使用权限校验", boolStr(isTrue(dto.AssistantRequirePermission)), "控制使用 AI 运维助手时是否校验 ai:assistant:use 权限码")
	_ = up(settings.AssistantMaxToolIterations, "AI助手最大工具轮次", intStr(clampPtr(dto.AssistantMaxToolIterations, 1, 20, 8)), "单轮对话最大工具调用轮次，防止工具调用失控")
	_ = up(settings.AssistantSystemPrompt, "AI助手系统提示词", trimOrDefault(dto.AssistantSystemPrompt, s.set.AssistantSystemPrompt(ctx)), "AI 运维助手的基础系统提示词")
	_ = up(settings.MCPClientEnabled, "AI MCP 客户端开关", boolStr(notFalse(dto.MCPClientEnabled)), "控制 AI 对话是否允许加载已启用的 MCP 服务")
	_ = up(settings.ChatHistoryWindow, "AI对话历史窗口", intStr(clampPtr(dto.ChatHistoryWindow, 2, 100, 20)), "每轮对话注入提示词的历史消息条数上限（2~100）")
	_ = up(settings.ReadonlySQLEnabled, "AI只读SQL工具开关", boolStr(notFalse(dto.ReadonlySQLEnabled)), "控制内置 execute_readonly_sql 工具是否注册给模型")
	_ = up(settings.ReadonlySQLPermissionCode, "AI只读SQL工具权限码", strings.TrimSpace(dto.ReadonlySQLPermissionCode), "调用内置 execute_readonly_sql 工具需要的权限码，留空则不校验")
	_ = up(settings.ReadonlySQLMaxRows, "AI只读SQL最大返回行数", intStr(clampPtr(dto.ReadonlySQLMaxRows, 1, 100, 100)), "内置 execute_readonly_sql 工具单次最多返回行数，代码层最大 100")
	_ = up(settings.ExecuteSQLEnabled, "AI执行SQL工具开关", boolStr(isTrue(dto.ExecuteSQLEnabled)), "控制内置 execute_sql 工具是否注册给模型")
	_ = up(settings.ExecuteSQLPermissionCode, "AI执行SQL工具权限码", strings.TrimSpace(dto.ExecuteSQLPermissionCode), "调用内置 execute_sql 工具需要的权限码，留空则不校验权限")
	_ = up(settings.ExecuteSQLMaxRows, "AI执行SQL最大返回行数", intStr(clampPtr(dto.ExecuteSQLMaxRows, 1, 500, 100)), "内置 execute_sql 工具查询语句单次最多返回行数，代码层最大 500")
	_ = up(settings.SchemaToolEnabled, "AI表结构工具开关", boolStr(notFalse(dto.SchemaToolEnabled)), "控制内置 describe_schema 工具是否注册给模型")
	_ = up(settings.SchemaToolPermissionCode, "AI表结构工具权限码", strings.TrimSpace(dto.SchemaToolPermissionCode), "调用内置 describe_schema 工具需要的权限码，留空则不校验")

	if err := s.saveRag(ctx, dto.Rag); err != nil {
		return err
	}

	if s.mcp != nil && oldMCP != notFalse(dto.MCPClientEnabled) {
		s.mcp.Reload()
	}
	return nil
}

func (s *Service) saveRag(ctx context.Context, rag *RagDTO) error {
	if rag == nil {
		return errs.New(40010, 400, "AI 配置不能为空")
	}
	if strings.TrimSpace(rag.CollectionName) == "" {
		return errs.New(40011, 400, "Qdrant 集合名不能为空")
	}
	if strings.TrimSpace(rag.QdrantURL) == "" {
		return errs.New(40012, 400, "Qdrant URL 不能为空")
	}
	if strings.TrimSpace(rag.EmbeddingModel) == "" {
		return errs.New(40013, 400, "Embedding 模型不能为空")
	}
	store := s.set.Store()
	_ = store.Upsert(ctx, settings.RagEnabled, "AI 知识库开关", boolStr(notFalse(rag.Enabled)), "控制 AI 知识库 / RAG 功能是否启用")
	_ = store.Upsert(ctx, settings.RagCollectionName, "AI 知识库 Qdrant 集合名", strings.TrimSpace(rag.CollectionName), "AI 知识库写入 Qdrant 时使用的集合名")
	_ = store.Upsert(ctx, settings.RagQdrantURL, "AI 知识库 Qdrant URL", strings.TrimSpace(rag.QdrantURL), "Qdrant REST 地址，例如 http://127.0.0.1:6333")
	_ = store.Upsert(ctx, settings.RagQdrantAPIKey, "AI 知识库 Qdrant API Key", resolveSecret(rag.QdrantAPIKey, s.set.RagQdrantAPIKey(ctx)), "Qdrant API Key，未开启鉴权时留空")
	_ = store.Upsert(ctx, settings.RagQdrantTimeoutMs, "AI 知识库 Qdrant 超时", intStr(clampPtr(rag.QdrantTimeoutMs, 1000, 120000, 5000)), "Qdrant 请求超时时间，单位毫秒")
	_ = store.Upsert(ctx, settings.RagEmbeddingBaseURL, "AI 知识库 Embedding Base URL", strings.TrimSpace(rag.EmbeddingBaseURL), "OpenAI 兼容 Embedding Base URL")
	_ = store.Upsert(ctx, settings.RagEmbeddingAPIKey, "AI 知识库 Embedding API Key", resolveSecret(rag.EmbeddingAPIKey, s.set.RagEmbeddingAPIKey(ctx)), "Embedding 服务 API Key")
	_ = store.Upsert(ctx, settings.RagEmbeddingModel, "AI 知识库 Embedding 模型", strings.TrimSpace(rag.EmbeddingModel), "Embedding 模型名称")
	_ = store.Upsert(ctx, settings.RagEmbeddingTimeoutMs, "AI 知识库 Embedding 超时", intStr(clampPtr(rag.EmbeddingTimeoutMs, 1000, 120000, 20000)), "Embedding 请求超时时间，单位毫秒")
	return nil
}

// ---- 小工具 ----

func notFalse(v *bool) bool { return v == nil || *v }
func isTrue(v *bool) bool   { return v != nil && *v }

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func intStr(v int) string {
	return strconv.Itoa(v)
}

func clampPtr(v *int, min, max, def int) int {
	n := def
	if v != nil {
		n = *v
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func trimOrDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func maskIfPresent(v string) string {
	if strings.TrimSpace(v) != "" {
		return mask
	}
	return ""
}

// resolveSecret：输入为空或仍是掩码则保留原值，否则采用新输入。
func resolveSecret(input, existing string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed != "" && trimmed != mask {
		return trimmed
	}
	return existing
}

// ---- Handler / 路由 ----

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Current(c *gin.Context) {
	response.Success(c, h.svc.Current(c.Request.Context()))
}

func (h *Handler) Save(c *gin.Context) {
	var dto DTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	if err := h.svc.Save(c.Request.Context(), &dto); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opWriter oplog.Writer) {
	g := rg.Group("/ai/config")
	g.GET("", h.Current)
	g.PUT("", middleware.OperationLog(opWriter, "AI 配置", oplog.BizUpdate), h.Save)
}
