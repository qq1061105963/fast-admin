package role

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[Role]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[Role](db)}
}

type Query struct {
	ID        string
	Name      string
	Status    *int
	Remark    string
	StartDate string
	EndDate   string
	Page      int
	Size      int
}

func (r *Repository) Page(ctx context.Context, q Query) ([]Role, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.ID != "" {
			db = db.Where("id = ?", q.ID)
		}
		if q.Name != "" {
			db = db.Where("name LIKE ?", "%"+q.Name+"%")
		}
		if q.Status != nil {
			db = db.Where("is_enabled = ?", *q.Status == 1)
		}
		if q.Remark != "" {
			db = db.Where("remark LIKE ?", "%"+q.Remark+"%")
		}
		if q.StartDate != "" {
			db = db.Where("DATE(created_at) >= ?", q.StartDate)
		}
		if q.EndDate != "" {
			db = db.Where("DATE(created_at) <= ?", q.EndDate)
		}
		return db.Order("created_at DESC, id")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *Repository) ListEnabled(ctx context.Context) ([]Role, error) {
	var list []Role
	err := r.DB.WithContext(ctx).Where("is_enabled = ?", true).Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *Repository) NameExists(ctx context.Context, id, name string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&Role{}).Where("name = ?", name)
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) ListByIDs(ctx context.Context, ids []string) ([]Role, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []Role
	err := r.DB.WithContext(ctx).Where("id IN ?", ids).Find(&list).Error
	return list, err
}

func (r *Repository) CodeExists(ctx context.Context, code string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&Role{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}
