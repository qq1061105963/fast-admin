package menu

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/permission"
)

type Service struct {
	repo     *Repository
	permRepo *permission.Repository
}

func NewService(repo *Repository, permRepo *permission.Repository) *Service {
	return &Service{repo: repo, permRepo: permRepo}
}

// UserMenuTree 返回当前用户可见的菜单树，不含按钮类型，供前端渲染导航菜单。
func (s *Service) UserMenuTree(ctx context.Context, userID string) ([]TreeNode, error) {
	list, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return buildTree(list, false), nil
}

// AllMenuTree 返回全量菜单树（含按钮），供管理端菜单维护页面使用。
func (s *Service) AllMenuTree(ctx context.Context) ([]TreeNode, error) {
	list, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return buildTree(list, true), nil
}

// PermissionCodes 返回用户所有角色对应菜单/按钮的 code 去重列表，供 /auth/codes 使用。
func (s *Service) PermissionCodes(ctx context.Context, userID string) ([]string, error) {
	list, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	seen := make(map[string]struct{}, len(list))
	codes := make([]string, 0, len(list))
	for _, m := range list {
		if m.Code == "" {
			continue
		}
		if _, ok := seen[m.Code]; ok {
			continue
		}
		seen[m.Code] = struct{}{}
		codes = append(codes, m.Code)
	}
	return codes, nil
}

// buildTree 对应 SysMenuService.buildMenuTree：isButton=false 时先过滤掉按钮类型，
// 再按 pid 挂树，找不到父节点的孤儿节点被静默丢弃；每层按 meta_order 升序、
// order 相同按 id（KSUID，天然按创建时间递增）排序。
func buildTree(list []Menu, isButton bool) []TreeNode {
	nodes := make(map[string]*TreeNode, len(list))
	order := make(map[string]int, len(list))
	var roots []string
	childrenOf := make(map[string][]string)

	for _, m := range list {
		if !isButton && m.Type == TypeButton {
			continue
		}
		node := toTreeNode(m)
		nodes[m.ID] = &node
		order[m.ID] = m.MetaOrder
		if m.PID == "" {
			roots = append(roots, m.ID)
		} else {
			childrenOf[m.PID] = append(childrenOf[m.PID], m.ID)
		}
	}

	var attach func(id string) TreeNode
	attach = func(id string) TreeNode {
		node := *nodes[id]
		childIDs := childrenOf[id]
		sort.SliceStable(childIDs, func(i, j int) bool {
			oi, oj := order[childIDs[i]], order[childIDs[j]]
			if oi == oj {
				return childIDs[i] < childIDs[j]
			}
			return orderValue(oi) < orderValue(oj)
		})
		for _, cid := range childIDs {
			if _, ok := nodes[cid]; !ok {
				continue // 父节点在过滤中被剔除（比如按钮）的孤儿场景，静默丢弃
			}
			node.Children = append(node.Children, attach(cid))
		}
		return node
	}

	sort.SliceStable(roots, func(i, j int) bool {
		oi, oj := order[roots[i]], order[roots[j]]
		if oi == oj {
			return roots[i] < roots[j]
		}
		return orderValue(oi) < orderValue(oj)
	})

	result := make([]TreeNode, 0, len(roots))
	for _, id := range roots {
		if _, ok := nodes[id]; !ok {
			continue
		}
		result = append(result, attach(id))
	}
	return result
}

func orderValue(o int) int {
	if o == 0 {
		return math.MaxInt32
	}
	return o
}

func (s *Service) NameExists(ctx context.Context, id, name string) (bool, error) {
	return s.repo.NameExists(ctx, id, name)
}

func (s *Service) PathExists(ctx context.Context, id, path string) (bool, error) {
	return s.repo.PathExists(ctx, id, path)
}

func (s *Service) Create(ctx context.Context, req *SaveRequest) (*Menu, error) {
	m, err := s.buildModel(ctx, "", req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return m, nil
}

func (s *Service) Update(ctx context.Context, req *SaveRequest) (*Menu, error) {
	if req.ID == "" {
		return nil, errs.ErrBadRequest
	}
	m, err := s.buildModel(ctx, req.ID, req)
	if err != nil {
		return nil, err
	}
	m.ID = req.ID
	if err := s.repo.Update(ctx, m); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return m, nil
}

func (s *Service) buildModel(ctx context.Context, id string, req *SaveRequest) (*Menu, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errs.ErrBadRequest
	}
	exists, err := s.repo.NameExists(ctx, id, req.Name)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40001, 400, "菜单名称已存在")
	}

	menuType, ok := stringToType[strings.ToUpper(req.Type)]
	if !ok {
		return nil, errs.New(40002, 400, "无效的菜单类型："+req.Type)
	}

	if menuType != TypeButton && req.Path != "" {
		exists, err := s.repo.PathExists(ctx, id, req.Path)
		if err != nil {
			return nil, errs.ErrInternal.Wrap(err)
		}
		if exists {
			return nil, errs.New(40003, 400, "路由路径已存在")
		}
	}

	if req.PID != "" {
		if req.PID == id {
			return nil, errs.New(40004, 400, "不能选择自己作为父菜单")
		}
		if _, err := s.repo.GetByID(ctx, req.PID); err != nil {
			return nil, errs.New(40005, 400, "父菜单不存在")
		}
		if id != "" {
			isDesc, err := s.isDescendant(ctx, req.PID, id)
			if err != nil {
				return nil, errs.ErrInternal.Wrap(err)
			}
			if isDesc {
				return nil, errs.New(40006, 400, "不能选择子孙节点作为父菜单")
			}
		}
	}

	status := int8(0)
	if req.Status == 1 {
		status = 1
	}

	return &Menu{
		PID: req.PID, Name: req.Name, Code: req.AuthCode, Type: menuType, Status: status,
		Path: req.Path, ActivePath: req.ActivePath, Component: req.Component, Remark: req.Remark,
		Icon: req.Meta.Icon, MetaActiveIcon: req.Meta.ActiveIcon, MetaTitle: req.Meta.Title,
		MetaOrder: req.Meta.Order, MetaAffixTab: req.Meta.AffixTab, MetaKeepAlive: req.Meta.KeepAlive,
		MetaHideInMenu: req.Meta.HideInMenu, MetaHideChildrenInMenu: req.Meta.HideChildrenInMenu,
		MetaHideInBreadcrumb: req.Meta.HideInBreadcrumb, MetaHideInTab: req.Meta.HideInTab,
		MetaBadge: req.Meta.Badge, MetaBadgeType: req.Meta.BadgeType, MetaBadgeVariants: req.Meta.BadgeVariants,
		MetaIframeSrc: req.Meta.IframeSrc, MetaLink: req.Meta.Link,
	}, nil
}

// isDescendant 判断 candidateID 是否是 targetID 的子孙节点，沿 pid 指针从
// candidate 往上走，最多 100 层防止环形数据死循环。
func (s *Service) isDescendant(ctx context.Context, candidateID, targetID string) (bool, error) {
	current := candidateID
	for i := 0; i < 100; i++ {
		m, err := s.repo.GetByID(ctx, current)
		if err != nil {
			return false, nil
		}
		if m.PID == "" {
			return false, nil
		}
		if m.PID == targetID {
			return true, nil
		}
		current = m.PID
	}
	return false, nil
}

// Delete 有子菜单则拒绝删除；成功删除后物理清理 sys_roles_menus 关联，
// 因为菜单本身是逻辑删除，数据库外键的 ON DELETE CASCADE 不会被触发。
func (s *Service) Delete(ctx context.Context, id string) error {
	hasChildren, err := s.repo.HasChildren(ctx, id)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if hasChildren {
		return errs.New(40007, 400, "该菜单存在子菜单，无法删除")
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if err := s.permRepo.DeleteRoleMenusByMenuID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
