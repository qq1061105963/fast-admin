// Package permission 管理 sys_roles_menus / sys_users_roles 两张纯中间表，
// 没有独立的 HTTP 接口，被 user/role/menu 模块复用。
package permission

// RoleMenu 对应 sys_roles_menus，组合主键，无审计字段/软删除。
type RoleMenu struct {
	RoleID string `gorm:"column:role_id;primaryKey"`
	MenuID string `gorm:"column:menu_id;primaryKey"`
}

func (RoleMenu) TableName() string { return "sys_roles_menus" }

// UserRole 对应 sys_users_roles，组合主键，无审计字段/软删除。
type UserRole struct {
	UserID string `gorm:"column:user_id;primaryKey"`
	RoleID string `gorm:"column:role_id;primaryKey"`
}

func (UserRole) TableName() string { return "sys_users_roles" }
