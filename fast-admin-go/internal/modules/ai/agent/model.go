package agent

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// AiChatSession 对应 ai_chat_session。
type AiChatSession struct {
	model.BaseModel
	SessionID string `gorm:"column:session_id" json:"sessionId"`
	UserID    string `gorm:"column:user_id" json:"userId"`
	Title     string `gorm:"column:title" json:"title"`
}

func (AiChatSession) TableName() string { return "ai_chat_session" }

// AiChatMessage 对应 ai_chat_message。
type AiChatMessage struct {
	model.BaseModel
	SessionID        string `gorm:"column:session_id" json:"sessionId"`
	Role             string `gorm:"column:role" json:"role"` // user / assistant
	Content          string `gorm:"column:content" json:"content"`
	ProcessJSON      string `gorm:"column:process_json" json:"processJson"`
	ModelName        string `gorm:"column:model_name" json:"modelName"`
	ModelProvider    string `gorm:"column:model_provider" json:"modelProvider"`
	ModelCode        string `gorm:"column:model_code" json:"modelCode"`
	PromptTokens     *int   `gorm:"column:prompt_tokens" json:"promptTokens"`
	CompletionTokens *int   `gorm:"column:completion_tokens" json:"completionTokens"`
	TotalTokens      *int   `gorm:"column:total_tokens" json:"totalTokens"`
}

func (AiChatMessage) TableName() string { return "ai_chat_message" }

// AiToolCallLog 对应 ai_tool_call_log。
type AiToolCallLog struct {
	model.BaseModel
	SessionID     string `gorm:"column:session_id" json:"sessionId"`
	OperatorID    string `gorm:"column:operator_id" json:"operatorId"`
	ToolName      string `gorm:"column:tool_name" json:"toolName"`
	Source        string `gorm:"column:source" json:"source"` // builtin / mcp
	ArgumentsJSON string `gorm:"column:arguments_json" json:"argumentsJson"`
	ResultJSON    string `gorm:"column:result_json" json:"resultJson"`
	Success       bool   `gorm:"column:success" json:"success"`
	ErrorMsg      string `gorm:"column:error_msg" json:"errorMsg"`
	CostMs        *int64 `gorm:"column:cost_ms" json:"costMs"`
}

func (AiToolCallLog) TableName() string { return "ai_tool_call_log" }

const (
	roleUser      = "user"
	roleAssistant = "assistant"
	sourceBuiltin = "builtin"
	sourceMCP     = "mcp"
)
