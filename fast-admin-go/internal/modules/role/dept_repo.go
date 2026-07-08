package role

import (
	"context"

	"gorm.io/gorm"
)

// DeptRepository 管理 sys_role_dept 中间表。
type DeptRepository struct {
	db *gorm.DB
}

func NewDeptRepository(db *gorm.DB) *DeptRepository {
	return &DeptRepository{db: db}
}

func (r *DeptRepository) WithTx(tx *gorm.DB) *DeptRepository {
	return &DeptRepository{db: tx}
}

func (r *DeptRepository) DeptIDsByRoleID(ctx context.Context, roleID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&RoleDept{}).Where("role_id = ?", roleID).Pluck("dept_id", &ids).Error
	return ids, err
}

// Replace 先删该角色的自定义部门绑定，DataScope=Custom 且 deptIDs 非空时再插入新绑定，
// 否则等于清空（对应 saveRoleDepts 的行为）。
func (r *DeptRepository) Replace(ctx context.Context, roleID string, scope DataScope, deptIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&RoleDept{}).Error; err != nil {
			return err
		}
		if scope != ScopeCustom || len(deptIDs) == 0 {
			return nil
		}
		rows := make([]RoleDept, 0, len(deptIDs))
		for _, deptID := range deptIDs {
			rows = append(rows, RoleDept{RoleID: roleID, DeptID: deptID})
		}
		return tx.Create(&rows).Error
	})
}

func (r *DeptRepository) DeleteByRoleID(ctx context.Context, roleID string) error {
	return r.db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&RoleDept{}).Error
}
