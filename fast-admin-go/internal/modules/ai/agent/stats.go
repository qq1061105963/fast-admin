package agent

import (
	"context"
	"time"
)

const maxStatDays = 90

// UsageStats 对应 AiUsageStatsDto。
type UsageStats struct {
	TotalMessages    int64        `json:"totalMessages"`
	PromptTokens     int64        `json:"promptTokens"`
	CompletionTokens int64        `json:"completionTokens"`
	TotalTokens      int64        `json:"totalTokens"`
	ByModel          []ModelUsage `json:"byModel"`
	ByDay            []DailyUsage `json:"byDay"`
}

type ModelUsage struct {
	ModelCode   string `json:"modelCode"`
	ModelName   string `json:"modelName"`
	Messages    int64  `json:"messages"`
	TotalTokens int64  `json:"totalTokens"`
}

type DailyUsage struct {
	Day         string `json:"day"`
	Messages    int64  `json:"messages"`
	TotalTokens int64  `json:"totalTokens"`
}

// StatsService 基于 ai_chat_message 的助手消息 token 聚合。
type StatsService struct {
	repo *Repository
}

func NewStatsService(repo *Repository) *StatsService { return &StatsService{repo: repo} }

func (s *StatsService) Stats(ctx context.Context, days int) (*UsageStats, error) {
	window := days
	if window < 1 {
		window = 1
	}
	if window > maxStatDays {
		window = maxStatDays
	}
	db := s.repo.DB().WithContext(ctx)

	var totals struct {
		Messages         int64
		PromptTokens     int64
		CompletionTokens int64
		TotalTokens      int64
	}
	if err := db.Model(&AiChatMessage{}).Where("role = ?", roleAssistant).
		Select("count(*) as messages, ifnull(sum(prompt_tokens),0) as prompt_tokens, ifnull(sum(completion_tokens),0) as completion_tokens, ifnull(sum(total_tokens),0) as total_tokens").
		Scan(&totals).Error; err != nil {
		return nil, err
	}

	var byModel []ModelUsage
	if err := db.Model(&AiChatMessage{}).Where("role = ? AND model_code IS NOT NULL", roleAssistant).
		Select("model_code as model_code, max(model_name) as model_name, count(*) as messages, ifnull(sum(total_tokens),0) as total_tokens").
		Group("model_code").Order("total_tokens DESC").Scan(&byModel).Error; err != nil {
		return nil, err
	}

	since := time.Now().AddDate(0, 0, -(window - 1))
	sinceStart := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	var byDay []DailyUsage
	if err := db.Model(&AiChatMessage{}).Where("role = ? AND created_at >= ?", roleAssistant, sinceStart).
		Select("date(created_at) as day, count(*) as messages, ifnull(sum(total_tokens),0) as total_tokens").
		Group("date(created_at)").Order("day ASC").Scan(&byDay).Error; err != nil {
		return nil, err
	}

	if byModel == nil {
		byModel = []ModelUsage{}
	}
	if byDay == nil {
		byDay = []DailyUsage{}
	}
	return &UsageStats{
		TotalMessages:    totals.Messages,
		PromptTokens:     totals.PromptTokens,
		CompletionTokens: totals.CompletionTokens,
		TotalTokens:      totals.TotalTokens,
		ByModel:          byModel,
		ByDay:            byDay,
	}, nil
}
