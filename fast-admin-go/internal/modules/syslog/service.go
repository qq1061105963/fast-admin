package syslog

import (
	"context"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/loginlog"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

type Service struct {
	opRepo    *OperationLogRepository
	loginRepo *LoginLogRepository
}

func NewService(opRepo *OperationLogRepository, loginRepo *LoginLogRepository) *Service {
	return &Service{opRepo: opRepo, loginRepo: loginRepo}
}

// Save 实现 framework/oplog.Writer：异步落库，写入失败只打日志，不影响主请求，
// 对应 Java 侧 OperationLogAspect 里 "记日志本身出错不能影响主流程" 的要求。
func (s *Service) Save(entry oplog.Entry) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.L().Sugar().Errorf("operation log panic: %v", r)
			}
		}()
		row := &OperationLog{
			Title: entry.Title, BusinessType: string(entry.BusinessType), Method: entry.Method,
			RequestMethod: entry.RequestMethod, OperatorType: entry.OperatorType, UserID: entry.UserID,
			Username: entry.Username, URL: entry.URL, IP: entry.IP, Location: entry.Location,
			RequestParams: entry.RequestParams, ResponseResult: entry.ResponseResult, Status: entry.Status,
			ErrorMsg: entry.ErrorMsg, CostTime: entry.CostTime,
		}
		row.CreatedAt = entry.CreatedAt
		if err := s.opRepo.Create(context.Background(), row); err != nil {
			logger.L().Sugar().Errorf("save operation log failed: %v", err)
		}
	}()
}

// SaveLogin 实现 framework/loginlog.Writer。
func (s *Service) SaveLogin(entry loginlog.Entry) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.L().Sugar().Errorf("login log panic: %v", r)
			}
		}()
		row := &LoginLog{
			UserID: entry.UserID, Username: entry.Username, IP: entry.IP, Location: entry.Location,
			Browser: entry.Browser, OS: entry.OS, Device: entry.Device, Status: entry.Status,
			Msg: entry.Msg, Type: string(entry.Type),
		}
		row.CreatedAt = entry.CreatedAt
		if err := s.loginRepo.Create(context.Background(), row); err != nil {
			logger.L().Sugar().Errorf("save login log failed: %v", err)
		}
	}()
}

// loginLogWriter 适配 loginlog.Writer 接口（方法名不同：Save vs SaveLogin，
// 因为 Service 本身要同时满足 oplog.Writer.Save，避免方法签名冲突）。
type loginLogWriter struct{ svc *Service }

func (w loginLogWriter) Save(entry loginlog.Entry) { w.svc.SaveLogin(entry) }

// AsLoginLogWriter 把 Service 包装成 loginlog.Writer。
func (s *Service) AsLoginLogWriter() loginlog.Writer {
	return loginLogWriter{svc: s}
}

func (s *Service) PageOperation(ctx context.Context, q OperationLogQuery) ([]OperationLog, int64, error) {
	list, total, err := s.opRepo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) DetailOperation(ctx context.Context, id string) (*OperationLog, error) {
	row, err := s.opRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	return row, nil
}

func (s *Service) DeleteOperation(ctx context.Context, id string) error {
	if err := s.opRepo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) CleanOperation(ctx context.Context) error {
	if err := s.opRepo.Clean(ctx); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) PageLogin(ctx context.Context, q LoginLogQuery) ([]LoginLog, int64, error) {
	list, total, err := s.loginRepo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) DeleteLogin(ctx context.Context, id string) error {
	if err := s.loginRepo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) CleanLogin(ctx context.Context) error {
	if err := s.loginRepo.Clean(ctx); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
