package tool

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[AiToolConfig]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[AiToolConfig](db)}
}

type Query struct {
	Name     string
	ToolCode string
	Type     string
	Enabled  *bool
	Page     int
	Size     int
}

// Page 按 enabled、created_at 倒序，对齐 Java orderByDesc("enabled").orderByDesc("created_at")。
func (r *Repository) Page(ctx context.Context, q Query) ([]AiToolConfig, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.Name != "" {
			db = db.Where("name LIKE ?", "%"+q.Name+"%")
		}
		if q.ToolCode != "" {
			db = db.Where("tool_code = ?", q.ToolCode)
		}
		if q.Type != "" {
			db = db.Where("type = ?", q.Type)
		}
		if q.Enabled != nil {
			db = db.Where("enabled = ?", *q.Enabled)
		}
		return db.Order("enabled DESC").Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *Repository) ToolCodeExists(ctx context.Context, excludeID, toolCode string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&AiToolConfig{}).Where("tool_code = ?", toolCode)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

// GetEnabledByToolCode 返回启用的非内置工具，无则 (nil, nil)。
func (r *Repository) GetEnabledByToolCode(ctx context.Context, toolCode string) (*AiToolConfig, error) {
	var t AiToolConfig
	err := r.DB.WithContext(ctx).
		Where("tool_code = ? AND enabled = ? AND system_builtin = ?", toolCode, true, false).
		Limit(1).First(&t).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListEnabled 返回全部启用的非内置工具，按 tool_code 升序。
func (r *Repository) ListEnabled(ctx context.Context) ([]AiToolConfig, error) {
	var list []AiToolConfig
	err := r.DB.WithContext(ctx).
		Where("enabled = ? AND system_builtin = ?", true, false).
		Order("tool_code ASC").Find(&list).Error
	return list, err
}

// SyncBuiltin 把内置 SQL 工具的 enabled/permission_code 同步为系统参数最新值。
func (r *Repository) SyncBuiltin(ctx context.Context, toolCode string, enabled bool, permissionCode string) error {
	return r.DB.WithContext(ctx).Model(&AiToolConfig{}).
		Where("tool_code = ? AND system_builtin = ?", toolCode, true).
		Updates(map[string]any{"enabled": enabled, "permission_code": permissionCode}).Error
}
