package file

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[File]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[File](db)}
}

type Query struct {
	Name        string
	StorageType string
	BizType     string
	BizID       string
	Ext         string
	Page        int
	Size        int
}

func (r *Repository) Page(ctx context.Context, q Query) ([]File, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.Name != "" {
			db = db.Where("original_name LIKE ?", "%"+q.Name+"%")
		}
		if q.StorageType != "" {
			db = db.Where("storage_type = ?", q.StorageType)
		}
		if q.BizType != "" {
			db = db.Where("biz_type = ?", q.BizType)
		}
		if q.BizID != "" {
			db = db.Where("biz_id = ?", q.BizID)
		}
		if q.Ext != "" {
			db = db.Where("ext = ?", q.Ext)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

// CountByConfigID 实现 fileconfig.FileReferenceCounter，供删除存储配置前做引用检查。
func (r *Repository) CountByConfigID(ctx context.Context, configID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&File{}).Where("config_id = ?", configID).Count(&count).Error
	return count, err
}
