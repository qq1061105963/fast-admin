package config

import (
	"context"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Page(ctx context.Context, q Query) ([]Config, int64, error) {
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) Detail(ctx context.Context, id string) (*Config, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	return c, nil
}

// GetValue 无缓存，直接查库，对应 Java 侧的实现现状。
func (s *Service) GetValue(ctx context.Context, key string) (string, error) {
	c, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return "", errs.ErrNotFound.Wrap(err)
	}
	return c.ConfigValue, nil
}

func (s *Service) Create(ctx context.Context, req *SaveRequest) (*Config, error) {
	exists, err := s.repo.KeyExists(ctx, "", req.ConfigKey)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40501, 400, "参数键名已存在")
	}
	c := &Config{
		ConfigName: req.ConfigName, ConfigKey: req.ConfigKey, ConfigValue: req.ConfigValue,
		ConfigType: req.ConfigType, Remark: req.Remark,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return c, nil
}

// Update 内置参数（config_type=1）只允许改 value/remark，其余字段静默忽略。
func (s *Service) Update(ctx context.Context, req *SaveRequest) (*Config, error) {
	if req.ID == "" {
		return nil, errs.ErrBadRequest
	}
	existing, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}

	if existing.ConfigType == TypeBuiltin {
		existing.ConfigValue = req.ConfigValue
		existing.Remark = req.Remark
	} else {
		exists, err := s.repo.KeyExists(ctx, req.ID, req.ConfigKey)
		if err != nil {
			return nil, errs.ErrInternal.Wrap(err)
		}
		if exists {
			return nil, errs.New(40501, 400, "参数键名已存在")
		}
		existing.ConfigName, existing.ConfigKey = req.ConfigName, req.ConfigKey
		existing.ConfigValue, existing.Remark = req.ConfigValue, req.Remark
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return existing, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	if c.ConfigType == TypeBuiltin {
		return errs.New(40502, 400, "系统内置参数不可删除")
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
