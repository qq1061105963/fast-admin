package agent

// SseEvent 对应 Java 侧的 AiChatSseEvent，前端消费的 text/event-stream 事件。
// type 枚举：session / thought / delta / tool / done / error。
type SseEvent struct {
	Type          string `json:"type"`
	SessionID     string `json:"sessionId,omitempty"`
	ModelName     string `json:"modelName,omitempty"`
	ModelProvider string `json:"modelProvider,omitempty"`
	ModelCode     string `json:"modelCode,omitempty"`
	Text          string `json:"text,omitempty"`
	Message       string `json:"message,omitempty"`
	MessageID     string `json:"messageId,omitempty"`
	ToolName      string `json:"toolName,omitempty"`
	Source        string `json:"source,omitempty"`
	Phase         string `json:"phase,omitempty"`
	Args          string `json:"args,omitempty"`
	Ok            *bool  `json:"ok,omitempty"`
	CostMs        *int64 `json:"costMs,omitempty"`
}

func sessionEvent(sessionID, modelName, modelProvider, modelCode string) SseEvent {
	return SseEvent{Type: "session", SessionID: sessionID, ModelName: modelName, ModelProvider: modelProvider, ModelCode: modelCode}
}

func thoughtEvent(text string) SseEvent { return SseEvent{Type: "thought", Text: text} }

func deltaEvent(text string) SseEvent { return SseEvent{Type: "delta", Text: text} }

func doneEvent(messageID string) SseEvent { return SseEvent{Type: "done", MessageID: messageID} }

func errorEvent(message string) SseEvent { return SseEvent{Type: "error", Message: message} }

// toolPending：execute_sql 执行前等待用户确认，MessageID 复用为 confirmToken，Args 放待执行 SQL。
func toolPendingEvent(toolName, sql, confirmToken string) SseEvent {
	return SseEvent{Type: "tool", MessageID: confirmToken, ToolName: toolName, Source: "builtin", Phase: "pending", Args: sql}
}

func toolStartEvent(toolName, source, args string) SseEvent {
	return SseEvent{Type: "tool", ToolName: toolName, Source: source, Phase: "start", Args: args}
}

func toolEndEvent(toolName, source string, ok bool, costMs int64) SseEvent {
	okv := ok
	cost := costMs
	return SseEvent{Type: "tool", ToolName: toolName, Source: source, Phase: "end", Ok: &okv, CostMs: &cost}
}
