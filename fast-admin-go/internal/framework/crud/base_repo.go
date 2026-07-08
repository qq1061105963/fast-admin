// Package crud 提供一个泛型的基础仓储实现，对应 MyBatis-Plus 的
// BaseMapper/IService：业务 repository 通过组合 BaseRepo[T] 直接获得
// 通用增删改查，只需要自己写特有的查询方法。
package crud

import (
	"context"

	"gorm.io/gorm"
)

// BaseRepo 是所有实体仓储的基类，T 必须是具体的 model 结构体（非指针）。
type BaseRepo[T any] struct {
	DB *gorm.DB
}

func NewBaseRepo[T any](db *gorm.DB) *BaseRepo[T] {
	return &BaseRepo[T]{DB: db}
}

func (r *BaseRepo[T]) Create(ctx context.Context, model *T) error {
	return r.DB.WithContext(ctx).Create(model).Error
}

func (r *BaseRepo[T]) Update(ctx context.Context, model *T) error {
	return r.DB.WithContext(ctx).Save(model).Error
}

func (r *BaseRepo[T]) DeleteByID(ctx context.Context, id any) error {
	var model T
	return r.DB.WithContext(ctx).Where("id = ?", id).Delete(&model).Error
}

// GetByID 显式用 "id = ?" 条件查询，不依赖 GORM First(&model, id) 的内联主键
// 推断——那套推断对字符串主键（我们的 KSUID）并不总是按预期工作，容易查出
// "记录不存在" 的假阴性。
func (r *BaseRepo[T]) GetByID(ctx context.Context, id any) (*T, error) {
	var model T
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// List 支持传入任意数量的 Scope（如 WHERE 条件、排序、数据权限过滤），
// 用法与 GORM 原生 Scopes 一致，方便和 datascope 包组合。
func (r *BaseRepo[T]) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]T, error) {
	var list []T
	if err := r.DB.WithContext(ctx).Scopes(scopes...).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Page 返回分页结果和总数，page 从 1 开始。
func (r *BaseRepo[T]) Page(ctx context.Context, page, size int, scopes ...func(*gorm.DB) *gorm.DB) ([]T, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	var total int64
	var model T
	countTx := r.DB.WithContext(ctx).Model(&model).Scopes(scopes...)
	if err := countTx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []T
	listTx := r.DB.WithContext(ctx).Scopes(scopes...).
		Offset((page - 1) * size).
		Limit(size)
	if err := listTx.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
