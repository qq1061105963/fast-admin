package mcp

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// AiMcpServer 对应 ai_mcp_server。运行时状态字段（connected/toolCount 等）不落库，
// 由 Manager 在返回前回填（gorm:"-"）。
type AiMcpServer struct {
	model.BaseModel
	Name              string `gorm:"column:name" json:"name"`
	Transport         string `gorm:"column:transport" json:"transport"` // stdio / sse / streamable-http
	Command           string `gorm:"column:command" json:"command"`
	URL               string `gorm:"column:url" json:"url"`
	ArgsJSON          string `gorm:"column:args_json" json:"argsJson"`
	HeadersJSON       string `gorm:"column:headers_json" json:"headersJson"`
	Enabled           bool   `gorm:"column:enabled" json:"enabled"`
	KeepAlive         bool   `gorm:"column:keep_alive" json:"keepAlive"`
	KeepAliveInterval int    `gorm:"column:keep_alive_interval" json:"keepAliveInterval"`
	KeepAliveJobID    string `gorm:"column:keep_alive_job_id" json:"keepAliveJobId"`
	Remark            string `gorm:"column:remark" json:"remark"`

	// 运行时状态（不落库）
	Connected         *bool  `gorm:"-" json:"connected"`
	ToolCount         *int   `gorm:"-" json:"toolCount"`
	PromptCount       *int   `gorm:"-" json:"promptCount"`
	ResourceCount     *int   `gorm:"-" json:"resourceCount"`
	ContextTokenCount *int   `gorm:"-" json:"contextTokenCount"`
	StatusMessage     string `gorm:"-" json:"statusMessage"`
}

func (AiMcpServer) TableName() string { return "ai_mcp_server" }
