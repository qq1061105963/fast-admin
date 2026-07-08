package role

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/permission"
)

type Service struct {
	repo     *Repository
	deptRepo *DeptRepository
	permRepo *permission.Repository
}

func NewService(repo *Repository, deptRepo *DeptRepository, permRepo *permission.Repository) *Service {
	return &Service{repo: repo, deptRepo: deptRepo, permRepo: permRepo}
}

func (s *Service) Page(ctx context.Context, q PageQuery) ([]Dto, int64, error) {
	var status *int
	if q.Status != "" {
		v := 0
		fmt.Sscanf(q.Status, "%d", &v)
		status = &v
	}
	list, total, err := s.repo.Page(ctx, Query{
		ID: q.ID, Name: q.Name, Status: status, Remark: q.Remark,
		StartDate: q.StartTime, EndDate: q.EndTime, Page: q.Page, Size: q.Size,
	})
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	dtos := make([]Dto, 0, len(list))
	for _, r := range list {
		dtos = append(dtos, toDto(r))
	}
	return dtos, total, nil
}

func (s *Service) Select(ctx context.Context) ([]SelectOption, error) {
	list, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	options := make([]SelectOption, 0, len(list))
	for _, r := range list {
		options = append(options, SelectOption{Label: r.Name, Value: r.ID})
	}
	return options, nil
}

func (s *Service) Detail(ctx context.Context, id string) (*Dto, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	dto := toDto(*r)
	dto.Permissions, err = s.permRepo.MenuIDsByRoleID(ctx, id)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	dto.DeptIDs, err = s.deptRepo.DeptIDsByRoleID(ctx, id)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return &dto, nil
}

func (s *Service) NameExists(ctx context.Context, id, name string) (bool, error) {
	return s.repo.NameExists(ctx, id, name)
}

// generateRoleCode 对应 Java 侧的自动生成规则："ROLE_" + 名称大写(空白转下划线)，
// 冲突则依次追加 _1/_2/... 直到唯一。
func (s *Service) generateRoleCode(ctx context.Context, name string) (string, error) {
	base := "ROLE_" + strings.ToUpper(strings.Join(strings.Fields(name), "_"))
	code := base
	for i := 1; ; i++ {
		exists, err := s.repo.CodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
		code = fmt.Sprintf("%s_%d", base, i)
	}
}

func (s *Service) Create(ctx context.Context, req *SaveRequest) (*Dto, error) {
	exists, err := s.repo.NameExists(ctx, "", req.Name)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40101, 400, "角色名称已存在")
	}

	scope := DataScope(req.DataScope)
	if scope == 0 {
		scope = ScopeAll
	}

	code, err := s.generateRoleCode(ctx, req.Name)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	r := &Role{
		Name: req.Name, Code: code, Remark: req.Remark,
		IsEnabled: req.Status == 1, DataScope: scope,
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if len(req.Permissions) > 0 {
		if err := s.permRepo.ReplaceRoleMenus(ctx, r.ID, req.Permissions); err != nil {
			return nil, errs.ErrInternal.Wrap(err)
		}
	}
	if err := s.deptRepo.Replace(ctx, r.ID, scope, req.DeptIDs); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	dto := toDto(*r)
	dto.Permissions, dto.DeptIDs = req.Permissions, req.DeptIDs
	return &dto, nil
}

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
		return nil, errs.New(40101, 400, "角色名称已存在")
	}

	scope := DataScope(req.DataScope)
	if scope == 0 {
		scope = ScopeAll
	}

	existing.Name = req.Name
	existing.Remark = req.Remark
	existing.IsEnabled = req.Status == 1
	existing.DataScope = scope
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if err := s.permRepo.ReplaceRoleMenus(ctx, req.ID, req.Permissions); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if err := s.deptRepo.Replace(ctx, req.ID, scope, req.DeptIDs); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	dto := toDto(*existing)
	dto.Permissions, dto.DeptIDs = req.Permissions, req.DeptIDs
	return &dto, nil
}

// Delete 用事务包裹主表删除 + 三张关联表清理，避免中间状态不一致。
func (s *Service) Delete(ctx context.Context, id string) error {
	err := s.repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&Role{}, "id = ?", id).Error; err != nil {
			return err
		}
		permTx := s.permRepo.WithTx(tx)
		if err := permTx.DeleteRoleMenusByRoleID(ctx, id); err != nil {
			return err
		}
		if err := permTx.DeleteUserRolesByRoleID(ctx, id); err != nil {
			return err
		}
		return s.deptRepo.WithTx(tx).DeleteByRoleID(ctx, id)
	})
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
