package fileconfig

import (
	"context"
	"encoding/json"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/file/storage"
)

// FileReferenceCounter 由 file 模块实现：统计有多少历史文件仍引用某个存储配置，
// 用来在删除配置前做保护，避免破坏依赖单向关系（fileconfig 不反向依赖 file 包）。
type FileReferenceCounter interface {
	CountByConfigID(ctx context.Context, configID string) (int64, error)
}

type Service struct {
	repo     *Repository
	factory  *storage.Factory
	fileRefs FileReferenceCounter
}

func NewService(repo *Repository, factory *storage.Factory) *Service {
	return &Service{repo: repo, factory: factory}
}

// SetFileReferenceCounter 在 bootstrap 里 file 模块初始化完成后回填，
// 打破 fileconfig <-> file 的构造顺序依赖。
func (s *Service) SetFileReferenceCounter(c FileReferenceCounter) {
	s.fileRefs = c
}

// ActiveConfig 实现 storage.ActiveConfigProvider。
func (s *Service) ActiveConfig(ctx context.Context) (string, storage.Type, []byte, string, error) {
	c, err := s.repo.GetActive(ctx)
	if err != nil {
		return "", "", nil, "", err
	}
	return c.ID, storage.Type(c.Type), []byte(c.RawConfig), c.URLPrefix, nil
}

func (s *Service) Page(ctx context.Context, q Query) ([]Dto, int64, error) {
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	dtos := make([]Dto, 0, len(list))
	for _, c := range list {
		dtos = append(dtos, toDto(c))
	}
	return dtos, total, nil
}

func (s *Service) Detail(ctx context.Context, id string) (*Dto, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	dto := toDto(*c)
	return &dto, nil
}

func validateConfigJSON(typ string, raw []byte) error {
	if _, err := storage.ParseConfig(storage.Type(typ), raw); err != nil {
		return errs.New(40601, 400, "存储配置格式不合法："+err.Error())
	}
	return nil
}

func (s *Service) Create(ctx context.Context, req *SaveRequest) (*Dto, error) {
	exists, err := s.repo.NameExists(ctx, "", req.Name)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40602, 400, "配置名称已存在")
	}

	rawJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, errs.ErrBadRequest.Wrap(err)
	}
	if err := validateConfigJSON(req.Type, rawJSON); err != nil {
		return nil, err
	}

	c := &Config{
		Name: req.Name, Type: req.Type, RawConfig: string(rawJSON),
		URLPrefix: req.URLPrefix, IsActive: false, Remark: req.Remark,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	dto := toDto(*c)
	return &dto, nil
}

// Update 不允许修改存储类型（建议新建配置）；敏感字段走脱敏合并；
// 如果被更新的配置正是当前激活配置，会让工厂缓存失效。
func (s *Service) Update(ctx context.Context, req *SaveRequest) (*Dto, error) {
	if req.ID == "" {
		return nil, errs.ErrBadRequest
	}
	existing, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}

	exists, err := s.repo.NameExists(ctx, req.ID, req.Name)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40602, 400, "配置名称已存在")
	}

	merged := mergeMaskedSecrets(req.Config, existing.RawConfig)
	rawJSON, err := json.Marshal(merged)
	if err != nil {
		return nil, errs.ErrBadRequest.Wrap(err)
	}
	if err := validateConfigJSON(existing.Type, rawJSON); err != nil {
		return nil, err
	}

	existing.Name = req.Name
	existing.RawConfig = string(rawJSON)
	existing.URLPrefix = req.URLPrefix
	existing.Remark = req.Remark
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if existing.IsActive {
		s.factory.Invalidate()
	}
	dto := toDto(*existing)
	return &dto, nil
}

func (s *Service) Activate(ctx context.Context, id string) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	if err := s.repo.Activate(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	s.factory.Invalidate()
	return nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	if c.IsActive {
		return errs.New(40603, 400, "激活中的配置不能删除")
	}
	if s.fileRefs != nil {
		count, err := s.fileRefs.CountByConfigID(ctx, id)
		if err != nil {
			return errs.ErrInternal.Wrap(err)
		}
		if count > 0 {
			return errs.New(40604, 400, "该配置仍被历史文件引用，不能删除")
		}
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
