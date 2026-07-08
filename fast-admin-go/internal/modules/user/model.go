package user

import (
	"time"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/model"
)

// Status 对应 SysUserStatus：0正常 1冻结 2锁定。
type Status int8

const (
	StatusNormal Status = 0
	StatusFrozen Status = 1
	StatusLocked Status = 2
)

// Sex 对应 SysUserSex：0未知 1男 2女。
type Sex int8

const (
	SexUnknown Sex = 0
	SexMale    Sex = 1
	SexFemale  Sex = 2
)

// User 对应 sys_user 表。
type User struct {
	model.BaseModel
	DeptID    string    `gorm:"column:dept_id" json:"deptId"`
	Username  string    `gorm:"column:username" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Email     string    `gorm:"column:email" json:"email"`
	Phone     string    `gorm:"column:phone" json:"phone"`
	Nickname  string    `gorm:"column:nickname" json:"nickname"`
	Avatar    string    `gorm:"column:avatar" json:"avatar"`
	Status    Status    `gorm:"column:status" json:"status"`
	Sex       Sex       `gorm:"column:sex" json:"sex"`
	LoginIP   string    `gorm:"column:login_ip" json:"loginIp"`
	LoginCity string    `gorm:"column:login_city" json:"loginCity"`
	LoginTime time.Time `gorm:"column:login_time" json:"loginTime"`
}

func (User) TableName() string { return "sys_user" }

// IsStatusValid 对应 AuthUserDto.isStatusValid：只有正常状态允许登录。
func (u *User) IsStatusValid() bool {
	return u.Status == StatusNormal
}

// StatusMessage 对应 AuthUserDto.getStatusMessage。
func (u *User) StatusMessage() string {
	switch u.Status {
	case StatusFrozen:
		return "您的账户已经被冻结, 请联系系统管理员."
	case StatusLocked:
		return "您的账户已经被锁定, 请联系系统管理员."
	default:
		return ""
	}
}
