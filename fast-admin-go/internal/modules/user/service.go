package user

import (
	"context"
	"fmt"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/auth"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/datascope"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/security"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/dept"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/permission"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/role"
)

type Service struct {
	repo     *Repository
	permRepo *permission.Repository
	roleRepo *role.Repository
	roleDept *role.DeptRepository
	deptSvc  *dept.Service
	tokenSvc *auth.TokenService
}

func NewService(repo *Repository, permRepo *permission.Repository, roleRepo *role.Repository, roleDept *role.DeptRepository, deptSvc *dept.Service, tokenSvc *auth.TokenService) *Service {
	return &Service{repo: repo, permRepo: permRepo, roleRepo: roleRepo, roleDept: roleDept, deptSvc: deptSvc, tokenSvc: tokenSvc}
}

// resolveDataScope 对应 DataScopeAspect：查当前用户所有角色，取并集计算可见范围。
func (s *Service) resolveDataScope(ctx context.Context, currentUserID string) (datascope.Filter, error) {
	current, err := s.repo.GetByID(ctx, currentUserID)
	if err != nil {
		return datascope.Filter{}, err
	}
	roleIDs, err := s.permRepo.RoleIDsByUserID(ctx, currentUserID)
	if err != nil {
		return datascope.Filter{}, err
	}
	if len(roleIDs) == 0 {
		return datascope.Filter{UserID: currentUserID}, nil
	}
	roles, err := s.roleRepo.ListByIDs(ctx, roleIDs)
	if err != nil {
		return datascope.Filter{}, err
	}

	deptSet := make(map[string]struct{})
	includeSelf := false
	for _, r := range roles {
		switch r.DataScope {
		case role.ScopeAll:
			return datascope.Filter{All: true}, nil
		case role.ScopeDeptAndSub:
			ids, err := s.deptSvc.GetDescendantIds(ctx, current.DeptID)
			if err != nil {
				return datascope.Filter{}, err
			}
			for _, id := range ids {
				deptSet[id] = struct{}{}
			}
		case role.ScopeDept:
			deptSet[current.DeptID] = struct{}{}
		case role.ScopeCustom:
			ids, err := s.roleDept.DeptIDsByRoleID(ctx, r.ID)
			if err != nil {
				return datascope.Filter{}, err
			}
			for _, id := range ids {
				deptSet[id] = struct{}{}
			}
		case role.ScopeSelf:
			includeSelf = true
		}
	}

	deptIDs := make([]string, 0, len(deptSet))
	for id := range deptSet {
		deptIDs = append(deptIDs, id)
	}
	return datascope.Filter{DeptIDs: deptIDs, IncludeSelf: includeSelf, UserID: currentUserID}, nil
}

// Page 分页查询用户列表，currentUserID 是发起查询的登录用户，用来计算数据权限范围。
func (s *Service) Page(ctx context.Context, currentUserID string, q PageQuery) ([]Dto, int64, error) {
	filter, err := s.resolveDataScope(ctx, currentUserID)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}

	var sex, status *int8
	if q.Sex != "" {
		var v int8
		fmt.Sscanf(q.Sex, "%d", &v)
		sex = &v
	}
	if q.Status != "" {
		var v int8
		fmt.Sscanf(q.Status, "%d", &v)
		status = &v
	}

	list, total, err := s.repo.Page(ctx, Query{
		ID: q.ID, DeptID: q.DeptID, Username: q.Username, Email: q.Email, Phone: q.Phone,
		Nickname: q.Nickname, Sex: sex, Status: status, LoginCity: q.LoginCity,
		StartDate: q.StartTime, EndDate: q.EndTime, Page: q.Page, Size: q.Size,
	}, datascope.Apply(filter))
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}

	dtos := make([]Dto, 0, len(list))
	for _, u := range list {
		d := toDto(u)
		if roleIDs, err := s.permRepo.RoleIDsByUserID(ctx, u.ID); err == nil {
			d.Roles = roleIDs
		}
		dtos = append(dtos, d)
	}
	return dtos, total, nil
}

func (s *Service) Info(ctx context.Context, userID string) (*InfoDto, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	dto := toInfoDto(*u)
	return &dto, nil
}

func (s *Service) Create(ctx context.Context, req *SaveRequest) (*Dto, error) {
	if req.Password == "" {
		return nil, errs.New(40301, 400, "密码不能为空")
	}
	exists, err := s.repo.UsernameExists(ctx, "", req.Username)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40302, 400, "用户名已存在")
	}
	hashed, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	u := &User{
		DeptID: req.DeptID, Username: req.Username, Password: hashed, Email: req.Email,
		Phone: req.Phone, Nickname: req.Nickname, Sex: Sex(req.Sex), Status: Status(req.Status),
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if len(req.Roles) > 0 {
		if err := s.permRepo.ReplaceUserRoles(ctx, u.ID, req.Roles); err != nil {
			return nil, errs.ErrInternal.Wrap(err)
		}
	}
	dto := toDto(*u)
	dto.Roles = req.Roles
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
	exists, err := s.repo.UsernameExists(ctx, req.ID, req.Username)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40302, 400, "用户名已存在")
	}

	existing.Username = req.Username
	existing.Email = req.Email
	existing.Phone = req.Phone
	existing.Nickname = req.Nickname
	existing.Sex = Sex(req.Sex)
	existing.Status = Status(req.Status)
	existing.DeptID = req.DeptID
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if err := s.permRepo.ReplaceUserRoles(ctx, req.ID, req.Roles); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	dto := toDto(*existing)
	dto.Roles = req.Roles
	return &dto, nil
}

// Delete 逻辑删除用户 + 物理清理角色关联 + 强制其所有会话下线。
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if err := s.permRepo.DeleteUserRolesByUserID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if s.tokenSvc != nil {
		_ = s.tokenSvc.KickoutUser(ctx, id)
	}
	return nil
}

// ChangePassword 对应 SysUserService.changePassword 的完整校验流程。
func (s *Service) ChangePassword(ctx context.Context, userID string, req *PasswordRequest) error {
	if len(req.NewPassword) < 6 {
		return errs.New(40303, 400, "新密码长度不能小于 6 位")
	}
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	if !security.VerifyPassword(req.OldPassword, u.Password) {
		return errs.New(40304, 400, "旧密码不正确")
	}
	if security.VerifyPassword(req.NewPassword, u.Password) {
		return errs.New(40305, 400, "新密码不能与旧密码相同")
	}
	hashed, err := security.HashPassword(req.NewPassword)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	u.Password = hashed
	if err := s.repo.Update(ctx, u); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

// UpdateProfile 对应 SysUserService.updateProfile：nickname 非空才改，
// 其余三个字段只要请求里带了这个 key（哪怕是空字符串）就更新。
func (s *Service) UpdateProfile(ctx context.Context, userID string, req *ProfileRequest) (*InfoDto, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errs.ErrNotFound.Wrap(err)
	}
	if req.Nickname != nil && *req.Nickname != "" {
		u.Nickname = *req.Nickname
	}
	if req.Email != nil {
		u.Email = *req.Email
	}
	if req.Phone != nil {
		u.Phone = *req.Phone
	}
	if req.Avatar != nil {
		u.Avatar = *req.Avatar
	}
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	dto := toInfoDto(*u)
	return &dto, nil
}

func (s *Service) ChangeAvatar(ctx context.Context, userID, avatar string) error {
	if avatar == "" {
		return errs.ErrBadRequest
	}
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}
	u.Avatar = avatar
	if err := s.repo.Update(ctx, u); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
