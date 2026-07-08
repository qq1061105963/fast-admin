package user

import "time"

// InfoDto 对应 SysUserInfoDto，/system/user/info 返回，刻意不含密码/状态/性别/部门等字段。
type InfoDto struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

func toInfoDto(u User) InfoDto {
	return InfoDto{ID: u.ID, Username: u.Username, Email: u.Email, Phone: u.Phone, Nickname: u.Nickname, Avatar: u.Avatar}
}

// Dto 对应 SysUserDto，列表/新增/编辑用，字段最全。
type Dto struct {
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Nickname   string    `json:"nickname"`
	Sex        int8      `json:"sex"`
	Status     int8      `json:"status"`
	LoginIP    string    `json:"loginIp,omitempty"`
	LoginCity  string    `json:"loginCity,omitempty"`
	LoginTime  time.Time `json:"loginTime,omitempty"`
	CreateTime time.Time `json:"createTime"`
	DeptID     string    `json:"deptId,omitempty"`
	DeptName   string    `json:"deptName,omitempty"`
	Roles      []string  `json:"roles,omitempty"`
}

func toDto(u User) Dto {
	return Dto{
		ID: u.ID, Username: u.Username, Email: u.Email, Phone: u.Phone, Nickname: u.Nickname,
		Sex: int8(u.Sex), Status: int8(u.Status), LoginIP: u.LoginIP, LoginCity: u.LoginCity,
		LoginTime: u.LoginTime, CreateTime: u.CreatedAt, DeptID: u.DeptID,
	}
}

// SaveRequest 对应新增/编辑请求体。编辑时故意不接受 password 字段，
// 避免 Java 现状里"编辑时传了密码会被明文覆盖、不重新加密"的那个坑，
// 改密码必须走 /system/user/password 专用接口。
type SaveRequest struct {
	ID       string   `json:"id"`
	Username string   `json:"username" binding:"required,max=50"`
	Password string   `json:"password"` // 仅新增时使用
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Nickname string   `json:"nickname"`
	Sex      int8     `json:"sex"`
	Status   int8     `json:"status"`
	DeptID   string   `json:"deptId"`
	Roles    []string `json:"roles"`
}

type PasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

type ProfileRequest struct {
	Nickname *string `json:"nickname"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
	Avatar   *string `json:"avatar"`
}

type PageQuery struct {
	ID        string `form:"id"`
	DeptID    string `form:"deptId"`
	Username  string `form:"username"`
	Email     string `form:"email"`
	Phone     string `form:"phone"`
	Nickname  string `form:"nickname"`
	Sex       string `form:"sex"`
	Status    string `form:"status"`
	LoginCity string `form:"loginCity"`
	StartTime string `form:"startTime"`
	EndTime   string `form:"endTime"`
	Page      int    `form:"page,default=1"`
	Size      int    `form:"pageSize,default=10"`
}
