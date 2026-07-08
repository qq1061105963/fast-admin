package permission

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// WithTx 返回一个绑定到指定事务的 Repository，供需要跨表事务的调用方
// （比如删除角色要同时清理 sys_roles_menus/sys_users_roles）组合使用。
func (r *Repository) WithTx(tx *gorm.DB) *Repository {
	return &Repository{db: tx}
}

// -------- role <-> menu --------

func (r *Repository) MenuIDsByRoleID(ctx context.Context, roleID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&RoleMenu{}).Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	return ids, err
}

func (r *Repository) RoleIDsByMenuID(ctx context.Context, menuID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&RoleMenu{}).Where("menu_id = ?", menuID).Pluck("role_id", &ids).Error
	return ids, err
}

// ReplaceRoleMenus 先删后插，对应 Java 侧"先删该角色所有权限记录再重新批量插入"。
func (r *Repository) ReplaceRoleMenus(ctx context.Context, roleID string, menuIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&RoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		rows := make([]RoleMenu, 0, len(menuIDs))
		for _, menuID := range menuIDs {
			rows = append(rows, RoleMenu{RoleID: roleID, MenuID: menuID})
		}
		return tx.Create(&rows).Error
	})
}

func (r *Repository) DeleteRoleMenusByRoleID(ctx context.Context, roleID string) error {
	return r.db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&RoleMenu{}).Error
}

func (r *Repository) DeleteRoleMenusByMenuID(ctx context.Context, menuID string) error {
	return r.db.WithContext(ctx).Where("menu_id = ?", menuID).Delete(&RoleMenu{}).Error
}

// -------- user <-> role --------

func (r *Repository) RoleIDsByUserID(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&UserRole{}).Where("user_id = ?", userID).Pluck("role_id", &ids).Error
	return ids, err
}

// ReplaceUserRoles 先删后插，对应 Java 侧用户角色关系维护策略。
func (r *Repository) ReplaceUserRoles(ctx context.Context, userID string, roleIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]UserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			rows = append(rows, UserRole{UserID: userID, RoleID: roleID})
		}
		return tx.Create(&rows).Error
	})
}

func (r *Repository) DeleteUserRolesByUserID(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&UserRole{}).Error
}

func (r *Repository) DeleteUserRolesByRoleID(ctx context.Context, roleID string) error {
	return r.db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&UserRole{}).Error
}
