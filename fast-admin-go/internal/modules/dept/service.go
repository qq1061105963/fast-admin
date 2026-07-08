package dept

import (
	"context"
	"sort"
	"strings"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Tree 返回全量部门树，按名称排序（sys_dept 没有排序字段）。
func (s *Service) Tree(ctx context.Context) ([]TreeNode, error) {
	list, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return buildTree(list), nil
}

// ListEnabled 返回启用部门的扁平列表，供下拉框使用。
func (s *Service) ListEnabled(ctx context.Context) ([]Dept, error) {
	list, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return list, nil
}

func buildTree(list []Dept) []TreeNode {
	nodes := make(map[string]*TreeNode, len(list))
	childrenOf := make(map[string][]string)
	var roots []string

	for _, d := range list {
		node := toTreeNode(d)
		nodes[d.ID] = &node
		if d.PID == "" {
			roots = append(roots, d.ID)
		} else {
			childrenOf[d.PID] = append(childrenOf[d.PID], d.ID)
		}
	}

	var attach func(id string) TreeNode
	attach = func(id string) TreeNode {
		node := *nodes[id]
		childIDs := childrenOf[id]
		sort.SliceStable(childIDs, func(i, j int) bool {
			return nodes[childIDs[i]].Name < nodes[childIDs[j]].Name
		})
		for _, cid := range childIDs {
			node.Children = append(node.Children, attach(cid))
		}
		return node
	}

	sort.SliceStable(roots, func(i, j int) bool {
		return nodes[roots[i]].Name < nodes[roots[j]].Name
	})

	result := make([]TreeNode, 0, len(roots))
	for _, id := range roots {
		result = append(result, attach(id))
	}
	return result
}

// GetDescendantIds 返回 rootDeptID 自身 + 所有后代部门 ID 的扁平列表，
// 供数据权限"本部门及以下"范围计算复用。
func (s *Service) GetDescendantIds(ctx context.Context, rootDeptID string) ([]string, error) {
	list, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	childrenOf := make(map[string][]string, len(list))
	for _, d := range list {
		if d.PID != "" {
			childrenOf[d.PID] = append(childrenOf[d.PID], d.ID)
		}
	}
	result := []string{rootDeptID}
	queue := []string{rootDeptID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, child := range childrenOf[id] {
			result = append(result, child)
			queue = append(queue, child)
		}
	}
	return result, nil
}

func (s *Service) NameExists(ctx context.Context, id, pid, name string) (bool, error) {
	return s.repo.NameExists(ctx, id, pid, name)
}

func (s *Service) Create(ctx context.Context, req *SaveRequest) (*Dept, error) {
	d, err := s.validate(ctx, "", req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return d, nil
}

func (s *Service) Update(ctx context.Context, req *SaveRequest) (*Dept, error) {
	if req.ID == "" {
		return nil, errs.ErrBadRequest
	}
	d, err := s.validate(ctx, req.ID, req)
	if err != nil {
		return nil, err
	}
	d.ID = req.ID
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return d, nil
}

func (s *Service) validate(ctx context.Context, id string, req *SaveRequest) (*Dept, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errs.ErrBadRequest
	}
	exists, err := s.repo.NameExists(ctx, id, req.PID, req.Name)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40201, 400, "部门名称已存在")
	}

	if req.PID != "" {
		if req.PID == id {
			return nil, errs.New(40202, 400, "不能选择自己作为上级部门")
		}
		if _, err := s.repo.GetByID(ctx, req.PID); err != nil {
			return nil, errs.New(40203, 400, "上级部门不存在")
		}
		if id != "" {
			isDesc, err := s.isDescendant(ctx, req.PID, id)
			if err != nil {
				return nil, errs.ErrInternal.Wrap(err)
			}
			if isDesc {
				return nil, errs.New(40204, 400, "不能选择子孙节点作为上级部门")
			}
		}
	}

	return &Dept{
		Name: req.Name, PID: req.PID, Status: req.Status,
		Remark: req.Remark, IsEnabled: req.IsEnabled,
	}, nil
}

func (s *Service) isDescendant(ctx context.Context, candidateID, targetID string) (bool, error) {
	current := candidateID
	for i := 0; i < 100; i++ {
		d, err := s.repo.GetByID(ctx, current)
		if err != nil || d.PID == "" {
			return false, nil
		}
		if d.PID == targetID {
			return true, nil
		}
		current = d.PID
	}
	return false, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	hasChildren, err := s.repo.HasChildren(ctx, id)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if hasChildren {
		return errs.New(40205, 400, "存在子部门，无法删除")
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
