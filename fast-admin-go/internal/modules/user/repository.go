package user

import (
	"context"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

type Repository struct {
	*crud.BaseRepo[User]
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[User](db)}
}

type Query struct {
	ID        string
	DeptID    string
	Username  string
	Email     string
	Phone     string
	Nickname  string
	Sex       *int8
	Status    *int8
	LoginCity string
	StartDate string
	EndDate   string
	Page      int
	Size      int
}

// Page 按查询条件 + 额外的数据权限 scope 分页查询用户。先分页锁定 sys_user 结果集，
// 角色/部门名称等关联信息由 Service 层再单独批量查询拼装，避免像 Java 那样
// 因为一对多 JOIN 导致 LIMIT 失效需要额外做子查询规避。
func (r *Repository) Page(ctx context.Context, q Query, extraScopes ...func(*gorm.DB) *gorm.DB) ([]User, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.ID != "" {
			db = db.Where("id = ?", q.ID)
		}
		if q.DeptID != "" {
			db = db.Where("dept_id = ?", q.DeptID)
		}
		if q.Username != "" {
			db = db.Where("username LIKE ?", "%"+q.Username+"%")
		}
		if q.Email != "" {
			db = db.Where("email LIKE ?", "%"+q.Email+"%")
		}
		if q.Phone != "" {
			db = db.Where("phone LIKE ?", "%"+q.Phone+"%")
		}
		if q.Nickname != "" {
			db = db.Where("nickname LIKE ?", "%"+q.Nickname+"%")
		}
		if q.Sex != nil {
			db = db.Where("sex = ?", *q.Sex)
		}
		if q.Status != nil {
			db = db.Where("status = ?", *q.Status)
		}
		if q.LoginCity != "" {
			db = db.Where("login_city = ?", q.LoginCity)
		}
		if q.StartDate != "" {
			db = db.Where("DATE(created_at) >= ?", q.StartDate)
		}
		if q.EndDate != "" {
			db = db.Where("DATE(created_at) <= ?", q.EndDate)
		}
		for _, extra := range extraScopes {
			db = extra(db)
		}
		return db.Order("created_at DESC, id")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *Repository) GetByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	if err := r.DB.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) UsernameExists(ctx context.Context, id, username string) (bool, error) {
	q := r.DB.WithContext(ctx).Model(&User{}).Where("username = ?", username)
	if id != "" {
		q = q.Where("id <> ?", id)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

// UpdateLoginInfo 登录成功后更新最后登录 IP/时间（Java 现状表里有列但登录逻辑
// 从未回写，这里顺手补上，字段本身已经预留）。
func (r *Repository) UpdateLoginInfo(ctx context.Context, userID, ip string) error {
	return r.DB.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Updates(map[string]any{"login_ip": ip, "login_time": gorm.Expr("NOW()")}).Error
}
