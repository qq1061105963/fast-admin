package file

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// File 对应 sys_file。StorageType/ConfigID 是上传时使用的存储配置的快照，
// 下载/删除都要用文件自己记录的这两个字段去取历史配置，而不是当前激活配置。
type File struct {
	model.BaseModel
	OriginalName string `gorm:"column:original_name" json:"originalName"`
	StorageKey   string `gorm:"column:storage_key" json:"storageKey"`
	URL          string `gorm:"column:url" json:"url"`
	Size         int64  `gorm:"column:size" json:"size"`
	ContentType  string `gorm:"column:content_type" json:"contentType"`
	Ext          string `gorm:"column:ext" json:"ext"`
	Hash         string `gorm:"column:hash" json:"hash"`
	StorageType  string `gorm:"column:storage_type" json:"storageType"`
	ConfigID     string `gorm:"column:config_id" json:"configId"`
	BizType      string `gorm:"column:biz_type" json:"bizType"`
	BizID        string `gorm:"column:biz_id" json:"bizId"`
}

func (File) TableName() string { return "sys_file" }
