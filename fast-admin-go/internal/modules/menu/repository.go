package menu

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[Menu]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[Menu](db)}
}

// ListAll 返回全部启用+禁用菜单（含按钮），供管理端"全量菜单树"接口使用。
func (r *Repository) ListAll(ctx context.Context) ([]Menu, error) {
	var list []Menu
	err := r.DB.WithContext(ctx).Order("meta_order ASC, id ASC").Find(&list).Error
	return list, err
}

// ListByUserID 对应 SysMenuMapper.selectAllByUserId：查当前用户所有角色关联的
// 菜单+按钮并集（未去重按钮类型，交给 buildTree 处理）。
func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Menu, error) {
	var list []Menu
	err := r.DB.WithContext(ctx).Distinct("sys_menu.*").
		Joins("INNER JOIN sys_roles_menus rm ON rm.menu_id = sys_menu.id").
		Joins("INNER JOIN sys_users_roles ur ON ur.role_id = rm.role_id").
		Where("ur.user_id = ? AND sys_menu.status = 1", userID).
		Order("sys_menu.meta_order ASC, sys_menu.id ASC").
		Find(&list).Error
	return list, err
}

func (r *Repository) NameExists(ctx context.Context, id, name string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&Menu{}).Where("name = ?", name)
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) PathExists(ctx context.Context, id, path string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&Menu{}).Where("path = ? AND path <> ''", path)
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) HasChildren(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&Menu{}).Where("pid = ?", id).Count(&count).Error
	return count > 0, err
}
