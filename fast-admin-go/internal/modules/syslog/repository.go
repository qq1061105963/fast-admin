package syslog

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type OperationLogRepository struct {
	*crud.BaseRepo[OperationLog]
}

func NewOperationLogRepository(db *gorm.DB) *OperationLogRepository {
	return &OperationLogRepository{BaseRepo: crud.NewBaseRepo[OperationLog](db)}
}

type OperationLogQuery struct {
	Title        string
	BusinessType string
	Username     string
	Status       *int8
	Page         int
	Size         int
}

func (r *OperationLogRepository) Page(ctx context.Context, q OperationLogQuery) ([]OperationLog, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.Title != "" {
			db = db.Where("title LIKE ?", "%"+q.Title+"%")
		}
		if q.BusinessType != "" {
			db = db.Where("business_type = ?", q.BusinessType)
		}
		if q.Username != "" {
			db = db.Where("username LIKE ?", "%"+q.Username+"%")
		}
		if q.Status != nil {
			db = db.Where("status = ?", *q.Status)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *OperationLogRepository) Clean(ctx context.Context) error {
	return r.DB.WithContext(ctx).Where("1 = 1").Delete(&OperationLog{}).Error
}

type LoginLogRepository struct {
	*crud.BaseRepo[LoginLog]
}

func NewLoginLogRepository(db *gorm.DB) *LoginLogRepository {
	return &LoginLogRepository{BaseRepo: crud.NewBaseRepo[LoginLog](db)}
}

type LoginLogQuery struct {
	Username string
	IP       string
	Status   *int8
	Type     string
	Page     int
	Size     int
}

func (r *LoginLogRepository) Page(ctx context.Context, q LoginLogQuery) ([]LoginLog, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.Username != "" {
			db = db.Where("username LIKE ?", "%"+q.Username+"%")
		}
		if q.IP != "" {
			db = db.Where("ip LIKE ?", "%"+q.IP+"%")
		}
		if q.Status != nil {
			db = db.Where("status = ?", *q.Status)
		}
		if q.Type != "" {
			db = db.Where("type = ?", q.Type)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *LoginLogRepository) Clean(ctx context.Context) error {
	return r.DB.WithContext(ctx).Where("1 = 1").Delete(&LoginLog{}).Error
}
