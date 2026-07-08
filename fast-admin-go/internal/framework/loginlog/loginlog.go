// Package loginlog 定义登录/登出日志条目和写入接口，被 auth 登录模块和
// modules/log 模块共用，避免登录模块直接依赖日志模块的具体实现。
package loginlog

import "time"

type Type string

const (
	TypeLogin  Type = "LOGIN"
	TypeLogout Type = "LOGOUT"
)

// Entry 对应 sys_login_log 表的一行。
type Entry struct {
	UserID    string
	Username  string
	IP        string
	Location  string
	Browser   string
	OS        string
	Device    string
	Status    int8 // 1成功 0失败
	Msg       string
	Type      Type
	CreatedAt time.Time
}

type Writer interface {
	Save(entry Entry)
}
