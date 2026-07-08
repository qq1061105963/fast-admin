package fileconfig

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// Config 对应 sys_file_config：存储驱动配置，config 列是 JSON 字符串，
// 具体结构因 Type 而异（见 internal/modules/file/storage 各 XxxConfig）。
type Config struct {
	model.BaseModel
	Name      string `gorm:"column:name" json:"name"`
	Type      string `gorm:"column:type" json:"type"` // LOCAL/OSS/S3/SFTP/FTP
	RawConfig string `gorm:"column:config" json:"-"`
	URLPrefix string `gorm:"column:url_prefix" json:"urlPrefix"`
	IsActive  bool   `gorm:"column:is_active" json:"isActive"`
	Remark    string `gorm:"column:remark" json:"remark"`
}

func (Config) TableName() string { return "sys_file_config" }
