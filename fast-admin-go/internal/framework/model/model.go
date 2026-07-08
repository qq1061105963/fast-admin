// Package model 提供所有业务实体共用的基类，对应 Java 侧的 BaseEntity：
// KSUID 主键、审计字段（创建/更新人和时间）、软删除。
package model

import (
	"time"

	"github.com/segmentio/ksuid"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/audit"
)

// BaseModel 对应现有 Java 项目所有业务主表的公共列。is_deleted 是 tinyint(1) 的
// 布尔软删除标记（不是时间戳），用 soft_delete.DeletedAt 插件按 flag 模式映射。
type BaseModel struct {
	ID        string                `gorm:"column:id;primaryKey;size:27" json:"id"`
	CreatedBy string                `gorm:"column:created_by" json:"createdBy"` // 创建人姓名
	CreatedID string                `gorm:"column:created_id" json:"createdId"` // 创建人ID
	CreatedAt time.Time             `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedBy string                `gorm:"column:updated_by" json:"updatedBy"`
	UpdatedID string                `gorm:"column:updated_id" json:"updatedId"`
	UpdatedAt time.Time             `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	IsDeleted soft_delete.DeletedAt `gorm:"column:is_deleted;softDelete:flag" json:"-"`
}

// BeforeCreate 生成 KSUID 主键并回填审计人信息，对应 CustomIdGenerator + MetaObjectHandler。
func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = ksuid.New().String()
	}
	userID, username := audit.FromContext(tx.Statement.Context)
	m.CreatedBy, m.CreatedID = username, userID
	m.UpdatedBy, m.UpdatedID = username, userID
	return nil
}

// BeforeUpdate 只回填更新人信息，不动创建人字段。
func (m *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	userID, username := audit.FromContext(tx.Statement.Context)
	m.UpdatedBy, m.UpdatedID = username, userID
	return nil
}

// LogModel 用于纯日志表（操作日志/登录日志/任务日志）：只有 KSUID 主键和创建时间，
// 没有软删除、没有更新人字段，删除是物理删除。
type LogModel struct {
	ID        string    `gorm:"column:id;primaryKey;size:32" json:"id"` // 列留了余量，实际生成的是 27 位 KSUID
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (m *LogModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = ksuid.New().String()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	return nil
}
