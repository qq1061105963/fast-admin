package dept

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// Dept 对应 sys_dept 表。没有排序字段，树的展示顺序按名称排序。
type Dept struct {
	model.BaseModel
	Name      string `gorm:"column:name" json:"name"`
	PID       string `gorm:"column:pid" json:"pid"`
	Status    bool   `gorm:"column:status" json:"status"`
	Remark    string `gorm:"column:remark" json:"remark"`
	IsEnabled bool   `gorm:"column:is_enabled" json:"isEnabled"`
}

func (Dept) TableName() string { return "sys_dept" }
