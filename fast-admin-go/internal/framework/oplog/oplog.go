// Package oplog 定义操作日志条目和写入接口，被 middleware.OperationLog 中间件
// 和 modules/log 模块共用：中间件负责采集，log 模块负责落库，避免中间件直接依赖
// 具体的 repository 实现。
package oplog

import (
	"regexp"
	"time"
)

// BusinessType 对应 Java 侧的 BusinessType 枚举。
type BusinessType string

const (
	BizOther       BusinessType = "OTHER"
	BizCreate      BusinessType = "CREATE"
	BizUpdate      BusinessType = "UPDATE"
	BizDelete      BusinessType = "DELETE"
	BizQuery       BusinessType = "QUERY"
	BizGrant       BusinessType = "GRANT"
	BizImport      BusinessType = "IMPORT"
	BizExport      BusinessType = "EXPORT"
	BizForceLogout BusinessType = "FORCE_LOGOUT"
	BizClean       BusinessType = "CLEAN"
)

// Entry 对应 sys_operation_log 表的一行。
type Entry struct {
	Title          string
	BusinessType   BusinessType
	Method         string
	RequestMethod  string
	OperatorType   string
	UserID         string
	Username       string
	URL            string
	IP             string
	Location       string
	RequestParams  string
	ResponseResult string
	Status         int8 // 1成功 0失败
	ErrorMsg       string
	CostTime       int64
	CreatedAt      time.Time
}

// Writer 由 modules/log 实现，负责异步落库。
type Writer interface {
	Save(entry Entry)
}

var (
	sensitiveJSONFieldRe = regexp.MustCompile(`(?i)"(api-?key|password|secret|authorization|cookie|access-?token|refresh-?token)"\s*:\s*"[^"]*"`)
	sensitiveTokenJSONRe = regexp.MustCompile(`(?i)"token"\s*:\s*"[^"]*"`)
	sensitiveQueryRe     = regexp.MustCompile(`(?i)(api-?key|password|secret|authorization|cookie|access-?token|refresh-?token|token)=[^&\s]*`)
	bearerRe             = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)
)

// MaskSensitive 依次应用四条脱敏规则，对应 Java 侧 OperationLogAspect.maskSensitive。
func MaskSensitive(s string) string {
	if s == "" {
		return s
	}
	s = sensitiveJSONFieldRe.ReplaceAllStringFunc(s, func(m string) string {
		idx := regexp.MustCompile(`:\s*"`).FindStringIndex(m)
		if idx == nil {
			return m
		}
		return m[:idx[1]] + "******\""
	})
	s = sensitiveTokenJSONRe.ReplaceAllString(s, `"token":"******"`)
	s = sensitiveQueryRe.ReplaceAllStringFunc(s, func(m string) string {
		eq := regexp.MustCompile(`=`).FindStringIndex(m)
		if eq == nil {
			return m
		}
		return m[:eq[0]] + "=******"
	})
	s = bearerRe.ReplaceAllString(s, "Bearer ******")
	return s
}

const maxResponseLen = 2000

// TruncateResponse 对应 Java 侧响应体超过 2000 字符截断的规则。
func TruncateResponse(s string) string {
	r := []rune(s)
	if len(r) <= maxResponseLen {
		return s
	}
	return string(r[:maxResponseLen]) + "...(truncated)"
}
