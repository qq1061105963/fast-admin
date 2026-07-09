package agent

import (
	"context"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
)

const resultMaxChars = 4000

// AuditLogger 写入工具调用审计日志（ai_tool_call_log），内置工具与 MCP 工具共用。写失败不影响主流程。
type AuditLogger struct {
	repo *Repository
}

func NewAuditLogger(repo *Repository) *AuditLogger { return &AuditLogger{repo: repo} }

func (a *AuditLogger) Write(ctx context.Context, sessionID, operatorID, toolName, source,
	argsJSON, resultJSON string, success bool, errorMsg string, costMs int64) {
	cost := costMs
	entity := &AiToolCallLog{
		SessionID: sessionID, OperatorID: operatorID, ToolName: toolName, Source: source,
		ArgumentsJSON: truncate(argsJSON), ResultJSON: truncate(resultJSON),
		Success: success, ErrorMsg: truncate(errorMsg), CostMs: &cost,
	}
	if err := a.repo.CreateToolLog(ctx, entity); err != nil {
		logger.L().Sugar().Warnf("写入工具调用审计日志失败 '%s': %v", toolName, err)
	}
}

func truncate(value string) string {
	r := []rune(value)
	if len(r) <= resultMaxChars {
		return value
	}
	return string(r[:resultMaxChars])
}
