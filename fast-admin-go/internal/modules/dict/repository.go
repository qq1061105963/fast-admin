package dict

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type TypeRepository struct {
	*crud.BaseRepo[Type]
}

func NewTypeRepository(db *gorm.DB) *TypeRepository {
	return &TypeRepository{BaseRepo: crud.NewBaseRepo[Type](db)}
}

type TypeQuery struct {
	DictName string
	DictType string
	Status   *int8
	Page     int
	Size     int
}

func (r *TypeRepository) Page(ctx context.Context, q TypeQuery) ([]Type, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.DictName != "" {
			db = db.Where("dict_name LIKE ?", "%"+q.DictName+"%")
		}
		if q.DictType != "" {
			db = db.Where("dict_type LIKE ?", "%"+q.DictType+"%")
		}
		if q.Status != nil {
			db = db.Where("status = ?", *q.Status)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *TypeRepository) ListEnabled(ctx context.Context) ([]Type, error) {
	var list []Type
	err := r.DB.WithContext(ctx).Where("status = 1").Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *TypeRepository) TypeExists(ctx context.Context, id, dictType string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&Type{}).Where("dict_type = ?", dictType)
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

type DataRepository struct {
	*crud.BaseRepo[Data]
}

func NewDataRepository(db *gorm.DB) *DataRepository {
	return &DataRepository{BaseRepo: crud.NewBaseRepo[Data](db)}
}

type DataQuery struct {
	DictType  string
	DictLabel string
	Status    *int8
	Page      int
	Size      int
}

func (r *DataRepository) Page(ctx context.Context, q DataQuery) ([]Data, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.DictType != "" {
			db = db.Where("dict_type = ?", q.DictType)
		}
		if q.DictLabel != "" {
			db = db.Where("dict_label LIKE ?", "%"+q.DictLabel+"%")
		}
		if q.Status != nil {
			db = db.Where("status = ?", *q.Status)
		}
		return db.Order("dict_sort ASC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *DataRepository) ListByType(ctx context.Context, dictType string) ([]Data, error) {
	var list []Data
	err := r.DB.WithContext(ctx).Where("dict_type = ? AND status = 1", dictType).
		Order("dict_sort ASC").Find(&list).Error
	return list, err
}

// DeleteByTypePhysical 级联物理删除某个字典类型下的全部字典数据，绕过软删除，
// 对应 Java 侧删除字典类型时用 mapper 直接 delete 而不是 removeById。
func (r *DataRepository) DeleteByTypePhysical(ctx context.Context, dictType string) error {
	return r.DB.WithContext(ctx).Unscoped().Where("dict_type = ?", dictType).Delete(&Data{}).Error
}
