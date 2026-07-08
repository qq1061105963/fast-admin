package dept

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[Dept]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[Dept](db)}
}

func (r *Repository) ListAll(ctx context.Context) ([]Dept, error) {
	var list []Dept
	err := r.DB.WithContext(ctx).Order("name ASC").Find(&list).Error
	return list, err
}

// ListEnabled 返回启用中的部门（status=1 且 is_enabled=1），供下拉框使用。
func (r *Repository) ListEnabled(ctx context.Context) ([]Dept, error) {
	var list []Dept
	err := r.DB.WithContext(ctx).Where("status = ? AND is_enabled = ?", true, true).
		Order("name ASC").Find(&list).Error
	return list, err
}

func (r *Repository) NameExists(ctx context.Context, id, pid, name string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&Dept{}).Where("name = ?", name)
	if pid == "" {
		q = q.Where("pid IS NULL OR pid = ''")
	} else {
		q = q.Where("pid = ?", pid)
	}
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) HasChildren(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&Dept{}).Where("pid = ?", id).Count(&count).Error
	return count > 0, err
}
