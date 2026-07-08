package job

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
)

// Scheduler 用 robfig/cron 承载调度骨架，任务定义仍然持久化在 sys_job 表，
// 启动时把 status=1 的任务全部读出来注册进内存调度器，达到"持久化定义、
// 内存调度"的效果——不照搬 Quartz 的 JDBC JobStore 那套集群协议表。
type Scheduler struct {
	cron     *cron.Cron
	registry *Registry
	logRepo  *LogRepository

	mu      sync.Mutex
	entries map[string]cron.EntryID
}

func NewScheduler(registry *Registry, logRepo *LogRepository) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		registry: registry,
		logRepo:  logRepo,
		entries:  make(map[string]cron.EntryID),
	}
}

func (s *Scheduler) Start() { s.cron.Start() }
func (s *Scheduler) Stop()  { <-s.cron.Stop().Done() }

// Schedule 注册/替换一个任务的调度：先移除旧条目（如果有），status!=1（暂停）
// 则只做移除不重新注册。
func (s *Scheduler) Schedule(j Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if old, ok := s.entries[j.ID]; ok {
		s.cron.Remove(old)
		delete(s.entries, j.ID)
	}
	if j.Status != 1 {
		return nil
	}

	var cronJob cron.Job = cron.FuncJob(func() { s.execute(context.Background(), j) })
	if !j.Concurrent {
		cronJob = cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(cronJob)
	}

	entryID, err := s.cron.AddJob(j.CronExpression, cronJob)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", j.CronExpression, err)
	}
	s.entries[j.ID] = entryID
	return nil
}

func (s *Scheduler) Unschedule(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.entries[jobID]; ok {
		s.cron.Remove(id)
		delete(s.entries, jobID)
	}
}

// RunOnce 立即异步执行一次，不影响原有的 cron 调度节奏。
func (s *Scheduler) RunOnce(j Job) {
	go s.execute(context.Background(), j)
}

// cronParser 和 cron.WithSeconds() 用的解析规格保持一致（6 段，含秒）。
var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// ValidateCron 只解析不注册，供新增/编辑任务时校验表达式合法性。
func ValidateCron(expr string) error {
	_, err := cronParser.Parse(expr)
	return err
}

func (s *Scheduler) execute(ctx context.Context, j Job) {
	start := time.Now()
	row := &JobLog{
		JobID: j.ID, JobName: j.JobName, JobGroup: j.JobGroup, BeanName: j.BeanName,
		MethodName: j.MethodName, MethodParams: j.MethodParams, Status: 2,
	}
	logInserted := s.logRepo.Create(ctx, row) == nil

	var execErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("panic: %v", r)
			}
		}()
		fn, ok := s.registry.Get(j.BeanName)
		if !ok {
			execErr = fmt.Errorf("job bean %q is not registered", j.BeanName)
			return
		}
		execErr = fn(ctx, j.MethodParams)
	}()

	row.CostTime = time.Since(start).Milliseconds()
	if execErr != nil {
		row.Status = 0
		row.ErrorMsg = execErr.Error()
	} else {
		row.Status = 1
	}

	if logInserted {
		if err := s.logRepo.Update(ctx, row); err != nil {
			logger.L().Sugar().Errorf("update job log failed: %v", err)
		}
	} else if err := s.logRepo.Create(ctx, row); err != nil {
		logger.L().Sugar().Errorf("insert job log failed: %v", err)
	}
}
