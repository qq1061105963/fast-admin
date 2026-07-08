package config

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// Config 对应 sys_config，简单的 key-value 系统参数表。
type Config struct {
	model.BaseModel
	ConfigName  string `gorm:"column:config_name" json:"configName"`
	ConfigKey   string `gorm:"column:config_key" json:"configKey"`
	ConfigValue string `gorm:"column:config_value" json:"configValue"`
	ConfigType  int8   `gorm:"column:config_type" json:"configType"` // 1=系统内置 0=自定义
	Remark      string `gorm:"column:remark" json:"remark"`
}

func (Config) TableName() string { return "sys_config" }

const TypeBuiltin int8 = 1
