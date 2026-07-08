package dict

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// Type 对应 sys_dict_type。
type Type struct {
	model.BaseModel
	DictName string `gorm:"column:dict_name" json:"dictName"`
	DictType string `gorm:"column:dict_type" json:"dictType"`
	Status   int8   `gorm:"column:status" json:"status"`
	Remark   string `gorm:"column:remark" json:"remark"`
}

func (Type) TableName() string { return "sys_dict_type" }

// Data 对应 sys_dict_data。dict_type 是字符串弱关联到 Type.DictType，没有外键。
type Data struct {
	model.BaseModel
	DictType  string `gorm:"column:dict_type" json:"dictType"`
	DictLabel string `gorm:"column:dict_label" json:"dictLabel"`
	DictValue string `gorm:"column:dict_value" json:"dictValue"`
	DictSort  int    `gorm:"column:dict_sort" json:"dictSort"`
	CSSClass  string `gorm:"column:css_class" json:"cssClass"`
	ListClass string `gorm:"column:list_class" json:"listClass"`
	IsDefault bool   `gorm:"column:is_default" json:"isDefault"`
	Status    int8   `gorm:"column:status" json:"status"`
	Remark    string `gorm:"column:remark" json:"remark"`
}

func (Data) TableName() string { return "sys_dict_data" }
