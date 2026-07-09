package agent

import (
	"context"
	"strings"

	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/settings"
)

const titleMax = 50

// HistoryService 负责会话与消息落库、历史加载，为多轮对话提供记忆。
type HistoryService struct {
	repo *Repository
	set  *settings.Settings
}

func NewHistoryService(repo *Repository, set *settings.Settings) *HistoryService {
	return &HistoryService{repo: repo, set: set}
}

// EnsureSession 确保会话存在，首条消息时以其为标题创建。
func (h *HistoryService) EnsureSession(ctx context.Context, sessionID, userID, firstMessage string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	existing, _ := h.repo.GetSession(ctx, sessionID)
	if existing != nil {
		return
	}
	_ = h.repo.CreateSession(ctx, &AiChatSession{
		SessionID: sessionID, UserID: userID, Title: buildTitle(firstMessage),
	})
}

// LoadHistory 取最近 N 条历史消息（按时间正序），供注入提示词。
func (h *HistoryService) LoadHistory(ctx context.Context, sessionID string) []chatMsg {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	recent, _ := h.repo.RecentMessages(ctx, sessionID, h.set.ChatHistoryWindow(ctx))
	// 反转成正序
	msgs := make([]chatMsg, 0, len(recent))
	for i := len(recent) - 1; i >= 0; i-- {
		item := recent[i]
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		role := roleUser
		if item.Role == roleAssistant {
			role = roleAssistant
		}
		msgs = append(msgs, chatMsg{Role: role, Content: item.Content})
	}
	return msgs
}

func (h *HistoryService) SaveUserMessage(ctx context.Context, sessionID, content string) {
	h.saveMessage(ctx, &AiChatMessage{SessionID: sessionID, Role: roleUser, Content: content})
}

func (h *HistoryService) SaveAssistantMessage(ctx context.Context, sessionID, content, processJSON,
	modelName, modelProvider, modelCode string, prompt, completion, total *int) {
	h.saveMessage(ctx, &AiChatMessage{
		SessionID: sessionID, Role: roleAssistant, Content: content, ProcessJSON: processJSON,
		ModelName: modelName, ModelProvider: modelProvider, ModelCode: modelCode,
		PromptTokens: prompt, CompletionTokens: completion, TotalTokens: total,
	})
}

func (h *HistoryService) saveMessage(ctx context.Context, m *AiChatMessage) {
	if strings.TrimSpace(m.SessionID) == "" || strings.TrimSpace(m.Content) == "" {
		return
	}
	_ = h.repo.CreateMessage(ctx, m)
}

func (h *HistoryService) ListSessions(ctx context.Context, userID string) ([]AiChatSession, error) {
	return h.repo.ListSessions(ctx, userID)
}

// ListMessages 只返回归属于该用户的会话消息。
func (h *HistoryService) ListMessages(ctx context.Context, sessionID, userID string) ([]AiChatMessage, error) {
	session, _ := h.repo.GetSession(ctx, sessionID)
	if session == nil || session.UserID != userID {
		return []AiChatMessage{}, nil
	}
	return h.repo.ListMessages(ctx, sessionID)
}

// DeleteSession 删除会话及其消息，仅限归属用户。
func (h *HistoryService) DeleteSession(ctx context.Context, sessionID, userID string) error {
	session, _ := h.repo.GetSession(ctx, sessionID)
	if session == nil || session.UserID != userID {
		return nil
	}
	if err := h.repo.DeleteSession(ctx, session.ID); err != nil {
		return err
	}
	return h.repo.DeleteMessagesBySession(ctx, sessionID)
}

func buildTitle(firstMessage string) string {
	trimmed := strings.TrimSpace(firstMessage)
	if trimmed == "" {
		return "新会话"
	}
	runes := []rune(trimmed)
	if len(runes) > titleMax {
		return string(runes[:titleMax])
	}
	return trimmed
}
