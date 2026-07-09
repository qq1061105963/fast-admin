package mcp

// InspectDTO 对应 AiMcpServerInspectDto：服务配置 + 运行时信息 + 工具/提示词/资源清单。
type InspectDTO struct {
	Server    *AiMcpServer   `json:"server"`
	Runtime   RuntimeInfo    `json:"runtime"`
	Tools     []ToolInfo     `json:"tools"`
	Prompts   []PromptInfo   `json:"prompts"`
	Resources []ResourceInfo `json:"resources"`
}

type RuntimeInfo struct {
	Connected         *bool          `json:"connected"`
	StatusMessage     string         `json:"statusMessage"`
	ToolCount         *int           `json:"toolCount"`
	PromptCount       *int           `json:"promptCount"`
	ResourceCount     *int           `json:"resourceCount"`
	ContextTokenCount *int           `json:"contextTokenCount"`
	Instructions      string         `json:"instructions"`
	ServerInfo        map[string]any `json:"serverInfo"`
	Capabilities      map[string]any `json:"capabilities"`
}

// ToolInfo/PromptInfo/ResourceInfo 的 json tag 与 MCP SDK 的对应结构一致，
// 便于用 JSON round-trip 从 SDK 类型解出，无需耦合具体字段名。
type ToolInfo struct {
	Name         string         `json:"name"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema"`
}

type PromptInfo struct {
	Name        string               `json:"name"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Arguments   []PromptArgumentInfo `json:"arguments"`
}

type PromptArgumentInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type ResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MIMEType    string `json:"mimeType"`
	Size        *int64 `json:"size"`
}
