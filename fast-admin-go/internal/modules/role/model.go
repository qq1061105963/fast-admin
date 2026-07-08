package role

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// DataScope 对应 DataScopeType：1全部 2本部门及以下 3本部门 4自定义 5仅本人。
type DataScope int8

const (
	ScopeAll        DataScope = 1
	ScopeDeptAndSub DataScope = 2
	ScopeDept       DataScope = 3
	ScopeCustom     DataScope = 4
	ScopeSelf       DataScope = 5
)

// Role 对应 sys_role 表。
type Role struct {
	model.BaseModel
	Name      string    `gorm:"column:name" json:"name"`
	Code      string    `gorm:"column:code" json:"code"`
	Remark    string    `gorm:"column:remark" json:"remark"`
	IsEnabled bool      `gorm:"column:is_enabled" json:"isEnabled"`
	DataScope DataScope `gorm:"column:data_scope" json:"dataScope"`
}

func (Role) TableName() string { return "sys_role" }

// RoleDept 对应 sys_role_dept，角色自定义数据范围绑定的部门（仅 DataScope=Custom 时有意义）。
type RoleDept struct {
	RoleID string `gorm:"column:role_id;primaryKey"`
	DeptID string `gorm:"column:dept_id;primaryKey"`
}

func (RoleDept) TableName() string { return "sys_role_dept" }
