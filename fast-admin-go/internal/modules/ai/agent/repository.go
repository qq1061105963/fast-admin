package agent

import (
	"context"

	"gorm.io/gorm"
)

// Repository 汇总会话 / 消息 / 工具调用日志三张表的读写。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ---- 会话 ----

func (r *Repository) GetSession(ctx context.Context, sessionID string) (*AiChatSession, error) {
	var s AiChatSession
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Limit(1).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) CreateSession(ctx context.Context, s *AiChatSession) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *Repository) ListSessions(ctx context.Context, userID string) ([]AiChatSession, error) {
	var list []AiChatSession
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at DESC").Find(&list).Error
	return list, err
}

func (r *Repository) DeleteSession(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&AiChatSession{}).Error
}

// ---- 消息 ----

func (r *Repository) CreateMessage(ctx context.Context, m *AiChatMessage) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// RecentMessages 取会话最近 limit 条消息，按创建时间倒序（调用方再反转成正序）。
func (r *Repository) RecentMessages(ctx context.Context, sessionID string, limit int) ([]AiChatMessage, error) {
	var list []AiChatMessage
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Order("created_at DESC").Limit(limit).Find(&list).Error
	return list, err
}

func (r *Repository) ListMessages(ctx context.Context, sessionID string) ([]AiChatMessage, error) {
	var list []AiChatMessage
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at ASC").Find(&list).Error
	return list, err
}

func (r *Repository) DeleteMessagesBySession(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&AiChatMessage{}).Error
}

// ---- 工具调用日志 ----

func (r *Repository) CreateToolLog(ctx context.Context, l *AiToolCallLog) error {
	return r.db.WithContext(ctx).Create(l).Error
}

type ToolLogQuery struct {
	ToolName   string
	Source     string
	Success    *bool
	SessionID  string
	OperatorID string
	Page       int
	Size       int
}

func (r *Repository) PageToolLog(ctx context.Context, q ToolLogQuery) ([]AiToolCallLog, int64, error) {
	tx := r.db.WithContext(ctx).Model(&AiToolCallLog{})
	if q.ToolName != "" {
		tx = tx.Where("tool_name LIKE ?", "%"+q.ToolName+"%")
	}
	if q.Source != "" {
		tx = tx.Where("source = ?", q.Source)
	}
	if q.Success != nil {
		tx = tx.Where("success = ?", *q.Success)
	}
	if q.SessionID != "" {
		tx = tx.Where("session_id = ?", q.SessionID)
	}
	if q.OperatorID != "" {
		tx = tx.Where("operator_id = ?", q.OperatorID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := q.Page, q.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	var list []AiToolCallLog
	err := tx.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *Repository) GetToolLog(ctx context.Context, id string) (*AiToolCallLog, error) {
	var l AiToolCallLog
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

// ---- 用量统计（对 ai_chat_message 的助手消息聚合）----

func (r *Repository) DB() *gorm.DB { return r.db }
