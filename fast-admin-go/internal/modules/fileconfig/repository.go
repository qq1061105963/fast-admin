package fileconfig

import (
	"context"
	"errors"

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
	Name string
	Type string
	Page int
	Size int
}

func (r *Repository) Page(ctx context.Context, q Query) ([]Config, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.Name != "" {
			db = db.Where("name LIKE ?", "%"+q.Name+"%")
		}
		if q.Type != "" {
			db = db.Where("type = ?", q.Type)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

var ErrNoActiveConfig = errors.New("fileconfig: no active storage config")

func (r *Repository) GetActive(ctx context.Context) (*Config, error) {
	var c Config
	err := r.DB.WithContext(ctx).Where("is_active = ?", true).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoActiveConfig
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) NameExists(ctx context.Context, id, name string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&Config{}).Where("name = ?", name)
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

// Activate 先把全表置为未激活，再激活目标，一个事务内完成。
func (r *Repository) Activate(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Config{}).Where("1 = 1").Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&Config{}).Where("id = ?", id).Update("is_active", true).Error
	})
}
