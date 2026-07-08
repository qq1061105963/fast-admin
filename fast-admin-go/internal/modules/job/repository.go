package job

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[Job]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[Job](db)}
}

type Query struct {
	JobName  string
	JobGroup string
	Status   *int8
	Page     int
	Size     int
}

func (r *Repository) Page(ctx context.Context, q Query) ([]Job, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.JobName != "" {
			db = db.Where("job_name LIKE ?", "%"+q.JobName+"%")
		}
		if q.JobGroup != "" {
			db = db.Where("job_group = ?", q.JobGroup)
		}
		if q.Status != nil {
			db = db.Where("status = ?", *q.Status)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *Repository) NameGroupExists(ctx context.Context, id, name, group string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&Job{}).Where("job_name = ? AND job_group = ?", name, group)
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

// ListEnabled 供启动时把 status=1 的任务全部注册进调度器。
func (r *Repository) ListEnabled(ctx context.Context) ([]Job, error) {
	var list []Job
	err := r.DB.WithContext(ctx).Where("status = 1").Find(&list).Error
	return list, err
}

type LogRepository struct {
	*crud.BaseRepo[JobLog]
}

func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{BaseRepo: crud.NewBaseRepo[JobLog](db)}
}

type LogQuery struct {
	JobID   string
	JobName string
	Status  *int8
	Page    int
	Size    int
}

func (r *LogRepository) Page(ctx context.Context, q LogQuery) ([]JobLog, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.JobID != "" {
			db = db.Where("job_id = ?", q.JobID)
		}
		if q.JobName != "" {
			db = db.Where("job_name LIKE ?", "%"+q.JobName+"%")
		}
		if q.Status != nil {
			db = db.Where("status = ?", *q.Status)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *LogRepository) Clean(ctx context.Context) error {
	return r.DB.WithContext(ctx).Where("1 = 1").Delete(&JobLog{}).Error
}
