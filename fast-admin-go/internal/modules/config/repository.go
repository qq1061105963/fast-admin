package config

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[Config]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[Config](db)}
}

type Query struct {
	ConfigName string
	ConfigKey  string
	ConfigType *int8
	Page       int
	Size       int
}

// Page 固定排除 config_key 以 "ai." 开头的记录：那部分参数是 AI 模块（不在本次
// 迁移范围）复用同一张表存自己的配置，管理界面要把它们排除掉不显示。
func (r *Repository) Page(ctx context.Context, q Query) ([]Config, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		db = db.Where("config_key NOT LIKE ?", "ai.%")
		if q.ConfigName != "" {
			db = db.Where("config_name LIKE ?", "%"+q.ConfigName+"%")
		}
		if q.ConfigKey != "" {
			db = db.Where("config_key LIKE ?", "%"+q.ConfigKey+"%")
		}
		if q.ConfigType != nil {
			db = db.Where("config_type = ?", *q.ConfigType)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *Repository) KeyExists(ctx context.Context, id, key string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&Config{}).Where("config_key = ?", key)
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) GetByKey(ctx context.Context, key string) (*Config, error) {
	var c Config
	if err := r.DB.WithContext(ctx).Where("config_key = ?", key).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}
