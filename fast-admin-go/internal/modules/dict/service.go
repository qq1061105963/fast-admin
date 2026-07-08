package dict

import (
	"context"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

type Service struct {
	typeRepo *TypeRepository
	dataRepo *DataRepository
}

func NewService(typeRepo *TypeRepository, dataRepo *DataRepository) *Service {
	return &Service{typeRepo: typeRepo, dataRepo: dataRepo}
}

func (s *Service) PageType(ctx context.Context, q TypeQuery) ([]Type, int64, error) {
	list, total, err := s.typeRepo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) ListEnabledTypes(ctx context.Context) ([]Type, error) {
	list, err := s.typeRepo.ListEnabled(ctx)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return list, nil
}

func (s *Service) CreateType(ctx context.Context, req *TypeSaveRequest) (*Type, error) {
	exists, err := s.typeRepo.TypeExists(ctx, "", req.DictType)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40401, 400, "字典类型编码已存在")
	}
	t := &Type{DictName: req.DictName, DictType: req.DictType, Status: statusValue(req.Status), Remark: req.Remark}
	if err := s.typeRepo.Create(ctx, t); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return t, nil
}

func (s *Service) UpdateType(ctx context.Context, req *TypeSaveRequest) (*Type, error) {
	if req.ID == "" {
		return nil, errs.ErrBadRequest
	}
	exists, err := s.typeRepo.TypeExists(ctx, req.ID, req.DictType)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40401, 400, "字典类型编码已存在")
	}
	existing, err := s.typeRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	existing.DictName, existing.DictType, existing.Status, existing.Remark = req.DictName, req.DictType, statusValue(req.Status), req.Remark
	if err := s.typeRepo.Update(ctx, existing); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return existing, nil
}

// DeleteType 级联物理删除该类型下全部字典数据，再逻辑删除类型本身。
func (s *Service) DeleteType(ctx context.Context, id string) error {
	t, err := s.typeRepo.GetByID(ctx, id)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	if err := s.dataRepo.DeleteByTypePhysical(ctx, t.DictType); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if err := s.typeRepo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) PageData(ctx context.Context, q DataQuery) ([]Data, int64, error) {
	list, total, err := s.dataRepo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) ListByType(ctx context.Context, dictType string) ([]Data, error) {
	list, err := s.dataRepo.ListByType(ctx, dictType)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return list, nil
}

func (s *Service) CreateData(ctx context.Context, req *DataSaveRequest) (*Data, error) {
	d := &Data{
		DictType: req.DictType, DictLabel: req.DictLabel, DictValue: req.DictValue, DictSort: req.DictSort,
		CSSClass: req.CSSClass, ListClass: req.ListClass, IsDefault: req.IsDefault,
		Status: statusValue(req.Status), Remark: req.Remark,
	}
	if err := s.dataRepo.Create(ctx, d); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return d, nil
}

func (s *Service) UpdateData(ctx context.Context, req *DataSaveRequest) (*Data, error) {
	if req.ID == "" {
		return nil, errs.ErrBadRequest
	}
	existing, err := s.dataRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	existing.DictType, existing.DictLabel, existing.DictValue = req.DictType, req.DictLabel, req.DictValue
	existing.DictSort, existing.CSSClass, existing.ListClass = req.DictSort, req.CSSClass, req.ListClass
	existing.IsDefault, existing.Status, existing.Remark = req.IsDefault, statusValue(req.Status), req.Remark
	if err := s.dataRepo.Update(ctx, existing); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return existing, nil
}

func (s *Service) DeleteData(ctx context.Context, id string) error {
	if err := s.dataRepo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func statusValue(status *int8) int8 {
	if status == nil {
		return 1
	}
	return *status
}
