package job

import (
	"context"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

type Service struct {
	repo      *Repository
	logRepo   *LogRepository
	scheduler *Scheduler
}

func NewService(repo *Repository, logRepo *LogRepository, scheduler *Scheduler) *Service {
	return &Service{repo: repo, logRepo: logRepo, scheduler: scheduler}
}

// Bootstrap 在应用启动时把全部 status=1 的任务注册进调度器，对应 Quartz
// auto-startup 时从 QRTZ_ 表恢复调度状态的效果。
func (s *Service) Bootstrap(ctx context.Context) error {
	jobs, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if err := s.scheduler.Schedule(j); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Page(ctx context.Context, q Query) ([]Job, int64, error) {
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) Detail(ctx context.Context, id string) (*Job, error) {
	j, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	return j, nil
}

func (s *Service) validate(ctx context.Context, id string, req *SaveRequest) (*Job, error) {
	if req.JobGroup == "" {
		req.JobGroup = "DEFAULT"
	}
	if req.MethodName == "" {
		req.MethodName = "execute"
	}
	if req.MisfirePolicy == 0 {
		req.MisfirePolicy = 2
	}

	if err := ValidateCron(req.CronExpression); err != nil {
		return nil, errs.New(40801, 400, "Cron 表达式不合法："+err.Error())
	}

	exists, err := s.repo.NameGroupExists(ctx, id, req.JobName, req.JobGroup)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40802, 400, "同分组下任务名称已存在")
	}

	return &Job{
		JobName: req.JobName, JobGroup: req.JobGroup, BeanName: req.BeanName,
		MethodName: req.MethodName, MethodParams: req.MethodParams, CronExpression: req.CronExpression,
		MisfirePolicy: req.MisfirePolicy, Concurrent: req.Concurrent, Status: req.Status, Remark: req.Remark,
	}, nil
}

// Create 新建任务默认暂停态（status 未传即 0），需要显式调用 Start 才会真正调度。
func (s *Service) Create(ctx context.Context, req *SaveRequest) (*Job, error) {
	j, err := s.validate(ctx, "", req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, j); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if err := s.scheduler.Schedule(*j); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return j, nil
}

// Update 先删后建重新注册调度，简单粗暴但稳定，对应 Java 侧的更新策略。
func (s *Service) Update(ctx context.Context, req *SaveRequest) (*Job, error) {
	if req.ID == "" {
		return nil, errs.ErrBadRequest
	}
	j, err := s.validate(ctx, req.ID, req)
	if err != nil {
		return nil, err
	}
	j.ID = req.ID
	if err := s.repo.Update(ctx, j); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if err := s.scheduler.Schedule(*j); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return j, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	s.scheduler.Unschedule(id)
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) Start(ctx context.Context, id string) error {
	j, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	j.Status = 1
	if err := s.repo.Update(ctx, j); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if err := s.scheduler.Schedule(*j); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) Pause(ctx context.Context, id string) error {
	j, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	j.Status = 0
	if err := s.repo.Update(ctx, j); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	s.scheduler.Unschedule(id)
	return nil
}

func (s *Service) RunOnce(ctx context.Context, id string) error {
	j, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	s.scheduler.RunOnce(*j)
	return nil
}

func (s *Service) PageLog(ctx context.Context, q LogQuery) ([]JobLog, int64, error) {
	list, total, err := s.logRepo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) DeleteLog(ctx context.Context, id string) error {
	if err := s.logRepo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) CleanLog(ctx context.Context) error {
	if err := s.logRepo.Clean(ctx); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
