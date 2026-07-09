// Package settings 提供 AI 模块运行期配置的读写，全部落在 sys_config 表里、以 "ai." 前缀
// 存储，对应 Java 侧的 AiAssistantSettingService + AiMcpSettingService + AiRagConfigService
// 共用的取值逻辑。Store 是对 sys_config 的最小封装，Settings 在其上提供带默认值与范围
// 约束的强类型访问器。
package settings

import (
	"context"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// sys_config 里 AI 配置项使用的 key。
const (
	AssistantEnabled           = "ai.assistant.enabled"
	AssistantRequirePermission = "ai.assistant.require-permission"
	AssistantMaxToolIterations = "ai.assistant.max-tool-iterations"
	AssistantSystemPrompt      = "ai.assistant.system-prompt"
	ReadonlySQLEnabled         = "ai.readonly-sql.enabled"
	ReadonlySQLPermissionCode  = "ai.readonly-sql.permission-code"
	ReadonlySQLMaxRows         = "ai.readonly-sql.max-rows"
	ExecuteSQLEnabled          = "ai.execute-sql.enabled"
	ExecuteSQLPermissionCode   = "ai.execute-sql.permission-code"
	ExecuteSQLMaxRows          = "ai.execute-sql.max-rows"
	SchemaToolEnabled          = "ai.schema-tool.enabled"
	SchemaToolPermissionCode   = "ai.schema-tool.permission-code"
	ChatHistoryWindow          = "ai.chat.history-window"
	MCPClientEnabled           = "ai.mcp.client.enabled"
	RagEnabled                 = "ai.rag.enabled"
	RagCollectionName          = "ai.rag.collection-name"
	RagQdrantURL               = "ai.rag.qdrant.url"
	RagQdrantAPIKey            = "ai.rag.qdrant.api-key"
	RagQdrantTimeoutMs         = "ai.rag.qdrant.timeout-ms"
	RagEmbeddingBaseURL        = "ai.rag.embedding.base-url"
	RagEmbeddingAPIKey         = "ai.rag.embedding.api-key"
	RagEmbeddingModel          = "ai.rag.embedding.model"
	RagEmbeddingTimeoutMs      = "ai.rag.embedding.timeout-ms"
)

// 默认值，对齐 Java 侧的各 DEFAULT_* 常量。
const (
	defaultSystemPrompt = "你是 Fast Admin 后台的 AI 运维助手。\n" +
		"回答要简洁、准确；当你无法确认后台事实时，明确说明需要工具或数据支持。\n" +
		"当前版本仅支持对话，不得声称已经执行后台写操作。"
	defaultReadonlySQLPermissionCode = "ai:sql:readonly"
	defaultExecuteSQLPermissionCode  = "ai:sql:execute"
	defaultSchemaToolPermissionCode  = "ai:sql:readonly"
	defaultEmbeddingModel            = "text-embedding-3-small"
	defaultCollectionName            = "fast_admin_rag"
)

// configRow 映射 sys_config 表，settings 只关心 key/value 与少量元信息，
// 不复用 modules/config.Config 以免引入循环依赖与审计钩子副作用。
type configRow struct {
	ID          string `gorm:"column:id;primaryKey"`
	ConfigName  string `gorm:"column:config_name"`
	ConfigKey   string `gorm:"column:config_key"`
	ConfigValue string `gorm:"column:config_value"`
	ConfigType  int8   `gorm:"column:config_type"`
	Remark      string `gorm:"column:remark"`
}

func (configRow) TableName() string { return "sys_config" }

// Store 直接读写 sys_config 表。GetValue 未命中返回 ("", false)。
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) GetValue(ctx context.Context, key string) (string, bool) {
	var row configRow
	err := s.db.WithContext(ctx).Where("config_key = ?", key).First(&row).Error
	if err != nil {
		return "", false
	}
	return row.ConfigValue, true
}

// Upsert 按 key 写入或更新，config_type 固定为 1（系统内置），id 用 key 把点替换成下划线，
// 对齐 Java 侧 AiConfigService.upsert 的实现。
func (s *Store) Upsert(ctx context.Context, key, name, value, remark string) error {
	var row configRow
	err := s.db.WithContext(ctx).Where("config_key = ?", key).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.WithContext(ctx).Create(&configRow{
			ID:          strings.ReplaceAll(key, ".", "_"),
			ConfigKey:   key,
			ConfigName:  name,
			ConfigValue: value,
			ConfigType:  1,
			Remark:      remark,
		}).Error
	}
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&configRow{}).Where("config_key = ?", key).
		Updates(map[string]any{
			"config_name":  name,
			"config_value": value,
			"config_type":  1,
			"remark":       remark,
		}).Error
}

// Settings 在 Store 之上提供强类型访问器。所有取值失败/越界都回退到默认值，绝不返回错误，
// 语义对齐 Java 侧的读取宽容策略。
type Settings struct {
	store *Store
}

func New(store *Store) *Settings { return &Settings{store: store} }

func (s *Settings) Store() *Store { return s.store }

func (s *Settings) getBool(ctx context.Context, key string, def bool) bool {
	v, ok := s.store.GetValue(ctx, key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func (s *Settings) getInt(ctx context.Context, key string, def int) int {
	v, ok := s.store.GetValue(ctx, key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

func (s *Settings) getString(ctx context.Context, key, def string) string {
	v, ok := s.store.GetValue(ctx, key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ---- 助手总开关 ----

func (s *Settings) AssistantEnabled(ctx context.Context) bool {
	return s.getBool(ctx, AssistantEnabled, true)
}

func (s *Settings) AssistantRequirePermission(ctx context.Context) bool {
	return s.getBool(ctx, AssistantRequirePermission, false)
}

func (s *Settings) AssistantMaxToolIterations(ctx context.Context) int {
	return clamp(s.getInt(ctx, AssistantMaxToolIterations, 8), 1, 20)
}

func (s *Settings) AssistantSystemPrompt(ctx context.Context) string {
	return s.getString(ctx, AssistantSystemPrompt, defaultSystemPrompt)
}

func (s *Settings) ChatHistoryWindow(ctx context.Context) int {
	return clamp(s.getInt(ctx, ChatHistoryWindow, 20), 2, 100)
}

// ---- 只读 SQL 工具 ----

func (s *Settings) ReadonlySQLEnabled(ctx context.Context) bool {
	return s.getBool(ctx, ReadonlySQLEnabled, true)
}

func (s *Settings) ReadonlySQLPermissionCode(ctx context.Context) string {
	return s.getString(ctx, ReadonlySQLPermissionCode, defaultReadonlySQLPermissionCode)
}

func (s *Settings) ReadonlySQLMaxRows(ctx context.Context) int {
	return clamp(s.getInt(ctx, ReadonlySQLMaxRows, 100), 1, 100)
}

// ---- 执行 SQL 工具 ----

func (s *Settings) ExecuteSQLEnabled(ctx context.Context) bool {
	return s.getBool(ctx, ExecuteSQLEnabled, false)
}

func (s *Settings) ExecuteSQLPermissionCode(ctx context.Context) string {
	return s.getString(ctx, ExecuteSQLPermissionCode, defaultExecuteSQLPermissionCode)
}

func (s *Settings) ExecuteSQLMaxRows(ctx context.Context) int {
	return clamp(s.getInt(ctx, ExecuteSQLMaxRows, 100), 1, 500)
}

// ---- 表结构工具 ----

func (s *Settings) SchemaToolEnabled(ctx context.Context) bool {
	return s.getBool(ctx, SchemaToolEnabled, true)
}

func (s *Settings) SchemaToolPermissionCode(ctx context.Context) string {
	return s.getString(ctx, SchemaToolPermissionCode, defaultSchemaToolPermissionCode)
}

// ---- MCP ----

func (s *Settings) MCPClientEnabled(ctx context.Context) bool {
	return s.getBool(ctx, MCPClientEnabled, true)
}

// ---- RAG ----

func (s *Settings) RagEnabled(ctx context.Context) bool {
	return s.getBool(ctx, RagEnabled, true)
}

func (s *Settings) RagCollectionName(ctx context.Context) string {
	return s.getString(ctx, RagCollectionName, defaultCollectionName)
}

func (s *Settings) RagQdrantURL(ctx context.Context) string {
	return s.getString(ctx, RagQdrantURL, "")
}

func (s *Settings) RagQdrantAPIKey(ctx context.Context) string {
	return s.getString(ctx, RagQdrantAPIKey, "")
}

func (s *Settings) RagQdrantTimeoutMs(ctx context.Context) int {
	return clamp(s.getInt(ctx, RagQdrantTimeoutMs, 5000), 500, 120000)
}

func (s *Settings) RagEmbeddingBaseURL(ctx context.Context) string {
	return s.getString(ctx, RagEmbeddingBaseURL, "")
}

func (s *Settings) RagEmbeddingAPIKey(ctx context.Context) string {
	return s.getString(ctx, RagEmbeddingAPIKey, "")
}

func (s *Settings) RagEmbeddingModel(ctx context.Context) string {
	return s.getString(ctx, RagEmbeddingModel, defaultEmbeddingModel)
}

func (s *Settings) RagEmbeddingTimeoutMs(ctx context.Context) int {
	return clamp(s.getInt(ctx, RagEmbeddingTimeoutMs, 20000), 1000, 120000)
}
