package tool

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// AiToolConfig 对应 ai_tool_config，后台维护的白名单工具（sql / http）。
type AiToolConfig struct {
	model.BaseModel
	Name           string `gorm:"column:name" json:"name"`
	ToolCode       string `gorm:"column:tool_code" json:"toolCode"`
	Type           string `gorm:"column:type" json:"type"` // sql / http
	Description    string `gorm:"column:description" json:"description"`
	Enabled        bool   `gorm:"column:enabled" json:"enabled"`
	SystemBuiltin  bool   `gorm:"column:system_builtin" json:"systemBuiltin"`
	PermissionCode string `gorm:"column:permission_code" json:"permissionCode"`
	Method         string `gorm:"column:method" json:"method"`
	URL            string `gorm:"column:url" json:"url"`
	HeadersJSON    string `gorm:"column:headers_json" json:"headersJson"`
	BodyTemplate   string `gorm:"column:body_template" json:"bodyTemplate"`
	SQLText        string `gorm:"column:sql_text" json:"sqlText"`
	ReadOnly       bool   `gorm:"column:read_only" json:"readOnly"`
	TimeoutMs      int    `gorm:"column:timeout_ms" json:"timeoutMs"`
	Remark         string `gorm:"column:remark" json:"remark"`
}

func (AiToolConfig) TableName() string { return "ai_tool_config" }

// 内置 SQL 工具的 tool_code。
const (
	ReadonlySQLToolCode = "execute_readonly_sql"
	ExecuteSQLToolCode  = "execute_sql"
	SchemaToolCode      = "describe_schema"
)
