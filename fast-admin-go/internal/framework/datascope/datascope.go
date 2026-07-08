// Package datascope 实现行级数据权限过滤，对应 fast-framework/datascope +
// fast-system/datascope 的 DataScopeAspect。Java 侧用 AOP 在查询前把过滤条件
// 注入 BaseQuery；Go 没有运行时 AOP，改为业务 Service 显式计算一个 Filter，
// 再作为 GORM Scope 传给 repository 查询方法。
package datascope

import (
	"strings"

	"gorm.io/gorm"
)

// Filter 描述一次查询应当应用的数据权限规则：
// All=true 时不过滤；否则按 "(dept_id IN (...) OR (IncludeSelf AND created_id = UserID))"
// 拼条件——多个角色的数据权限范围是取并集，不是互斥的，这里的字段就是并集计算后的结果。
type Filter struct {
	All         bool
	DeptIDs     []string
	IncludeSelf bool
	UserID      string
	DeptColumn  string // 默认 "dept_id"
	UserColumn  string // 默认 "created_id"
}

// Apply 返回一个 GORM Scope 函数，直接传给 Scopes(...) 使用：
//
//	repo.List(ctx, datascope.Apply(filter))
func Apply(f Filter) func(*gorm.DB) *gorm.DB {
	deptColumn := f.DeptColumn
	if deptColumn == "" {
		deptColumn = "dept_id"
	}
	userColumn := f.UserColumn
	if userColumn == "" {
		userColumn = "created_id"
	}

	return func(db *gorm.DB) *gorm.DB {
		if f.All {
			return db
		}

		var conds []string
		var args []any
		if len(f.DeptIDs) > 0 {
			conds = append(conds, deptColumn+" IN ?")
			args = append(args, f.DeptIDs)
		}
		if f.IncludeSelf && f.UserID != "" {
			conds = append(conds, userColumn+" = ?")
			args = append(args, f.UserID)
		}
		if len(conds) == 0 {
			// 没有任何可见范围时，返回恒假条件而不是全表扫描。
			return db.Where("1 = 0")
		}
		return db.Where("("+strings.Join(conds, " OR ")+")", args...)
	}
}
