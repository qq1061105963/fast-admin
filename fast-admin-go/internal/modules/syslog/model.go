// Package syslog 落地操作日志和登录日志：两张纯日志表，物理删除，没有审计
// 字段/软删除，实现 framework/oplog.Writer 和 framework/loginlog.Writer 接口
// 供其它模块异步写入。
package syslog

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

type OperationLog struct {
	model.LogModel
	Title          string `gorm:"column:title" json:"title"`
	BusinessType   string `gorm:"column:business_type" json:"businessType"`
	Method         string `gorm:"column:method" json:"method"`
	RequestMethod  string `gorm:"column:request_method" json:"requestMethod"`
	OperatorType   string `gorm:"column:operator_type" json:"operatorType"`
	UserID         string `gorm:"column:user_id" json:"userId"`
	Username       string `gorm:"column:username" json:"username"`
	URL            string `gorm:"column:url" json:"url"`
	IP             string `gorm:"column:ip" json:"ip"`
	Location       string `gorm:"column:location" json:"location"`
	RequestParams  string `gorm:"column:request_params" json:"requestParams"`
	ResponseResult string `gorm:"column:response_result" json:"responseResult"`
	Status         int8   `gorm:"column:status" json:"status"`
	ErrorMsg       string `gorm:"column:error_msg" json:"errorMsg"`
	CostTime       int64  `gorm:"column:cost_time" json:"costTime"`
}

func (OperationLog) TableName() string { return "sys_operation_log" }

type LoginLog struct {
	model.LogModel
	UserID   string `gorm:"column:user_id" json:"userId"`
	Username string `gorm:"column:username" json:"username"`
	IP       string `gorm:"column:ip" json:"ip"`
	Location string `gorm:"column:location" json:"location"`
	Browser  string `gorm:"column:browser" json:"browser"`
	OS       string `gorm:"column:os" json:"os"`
	Device   string `gorm:"column:device" json:"device"`
	Status   int8   `gorm:"column:status" json:"status"`
	Msg      string `gorm:"column:msg" json:"msg"`
	Type     string `gorm:"column:type" json:"type"`
}

func (LoginLog) TableName() string { return "sys_login_log" }
