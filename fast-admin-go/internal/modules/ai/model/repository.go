package model

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[AiModelConfig]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[AiModelConfig](db)}
}

type Query struct {
	Name     string
	Provider string
	Enabled  *bool
	Active   *bool
	Page     int
	Size     int
}

// Page 按 active、created_at 倒序，对齐 Java 侧 orderByDesc("active").orderByDesc("created_at")。
func (r *Repository) Page(ctx context.Context, q Query) ([]AiModelConfig, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.Name != "" {
			db = db.Where("name LIKE ?", "%"+q.Name+"%")
		}
		if q.Provider != "" {
			db = db.Where("provider = ?", q.Provider)
		}
		if q.Enabled != nil {
			db = db.Where("enabled = ?", *q.Enabled)
		}
		if q.Active != nil {
			db = db.Where("active = ?", *q.Active)
		}
		return db.Order("active DESC").Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *Repository) NameExists(ctx context.Context, excludeID, name string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&AiModelConfig{}).Where("name = ?", name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

// GetActiveEnabled 返回当前激活且启用的模型，无则返回 (nil, nil)。
func (r *Repository) GetActiveEnabled(ctx context.Context) (*AiModelConfig, error) {
	var m AiModelConfig
	err := r.DB.WithContext(ctx).Where("active = ? AND enabled = ?", true, true).Limit(1).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ClearActive 把所有配置的 active 清 0。
func (r *Repository) ClearActive(ctx context.Context) error {
	return r.DB.WithContext(ctx).Model(&AiModelConfig{}).Where("active = ?", true).Update("active", false).Error
}

// SetActive 把指定配置置为激活。
func (r *Repository) SetActive(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Model(&AiModelConfig{}).Where("id = ?", id).Update("active", true).Error
}

// RecordTestResult 记录一次连通性测试结果，latency 为 nil 表示失败。
func (r *Repository) RecordTestResult(ctx context.Context, id string, latency *int64, ok bool) error {
	now := time.Now()
	return r.DB.WithContext(ctx).Model(&AiModelConfig{}).Where("id = ?", id).Updates(map[string]any{
		"last_latency_ms": latency,
		"last_test_ok":    ok,
		"last_tested_at":  now,
	}).Error
}
