package mcp

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[AiMcpServer]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[AiMcpServer](db)}
}

type Query struct {
	Name      string
	Transport string
	Enabled   *bool
	Page      int
	Size      int
}

func (r *Repository) Page(ctx context.Context, q Query) ([]AiMcpServer, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.Name != "" {
			db = db.Where("name LIKE ?", "%"+q.Name+"%")
		}
		if q.Transport != "" {
			db = db.Where("transport = ?", q.Transport)
		}
		if q.Enabled != nil {
			db = db.Where("enabled = ?", *q.Enabled)
		}
		return db.Order("enabled DESC").Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *Repository) NameExists(ctx context.Context, excludeID, name string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&AiMcpServer{}).Where("name = ?", name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) ListEnabled(ctx context.Context) ([]AiMcpServer, error) {
	var list []AiMcpServer
	err := r.DB.WithContext(ctx).Where("enabled = ?", true).Find(&list).Error
	return list, err
}
