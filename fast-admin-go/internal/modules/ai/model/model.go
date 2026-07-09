package model

import (
	"time"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/model"
)

// AiModelConfig 对应 ai_model_config，后台维护的可选大模型配置。
type AiModelConfig struct {
	model.BaseModel
	Name          string     `gorm:"column:name" json:"name"`
	Provider      string     `gorm:"column:provider" json:"provider"` // anthropic / openai / openai-compatible
	Model         string     `gorm:"column:model" json:"model"`
	BaseURL       string     `gorm:"column:base_url" json:"baseUrl"`
	APIKey        string     `gorm:"column:api_key" json:"apiKey"`
	Enabled       bool       `gorm:"column:enabled" json:"enabled"`
	Active        bool       `gorm:"column:active" json:"active"`
	Temperature   *float64   `gorm:"column:temperature" json:"temperature"`
	MaxTokens     *int       `gorm:"column:max_tokens" json:"maxTokens"`
	Remark        string     `gorm:"column:remark" json:"remark"`
	LastLatencyMs *int64     `gorm:"column:last_latency_ms" json:"lastLatencyMs"`
	LastTestOk    *bool      `gorm:"column:last_test_ok" json:"lastTestOk"`
	LastTestedAt  *time.Time `gorm:"column:last_tested_at" json:"lastTestedAt"`
}

func (AiModelConfig) TableName() string { return "ai_model_config" }
