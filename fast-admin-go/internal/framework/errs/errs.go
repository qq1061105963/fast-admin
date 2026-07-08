// Package errs 定义统一的业务错误类型，替代 Java 侧的
// GlobalExceptionHandler + 自定义异常体系。
package errs

import "fmt"

// AppError 是所有业务/系统错误的载体，Code 用于返回给前端区分错误类型，
// HTTPStatus 决定响应状态码。
type AppError struct {
	Code       int    // 业务错误码
	Message    string // 面向用户的提示
	HTTPStatus int    // HTTP 状态码
	cause      error
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.cause
}

// Wrap 附加底层错误原因，用于日志记录，不改变返回给前端的 Message。
func (e *AppError) Wrap(cause error) *AppError {
	return &AppError{Code: e.Code, Message: e.Message, HTTPStatus: e.HTTPStatus, cause: cause}
}

func New(code int, httpStatus int, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: httpStatus}
}

// 预置的通用错误码，业务模块可以在此基础上扩展自己的错误码区间。
var (
	ErrBadRequest   = New(40000, 400, "请求参数错误")
	ErrUnauthorized = New(40100, 401, "未登录或登录已失效")
	ErrForbidden    = New(40300, 403, "没有操作权限")
	ErrNotFound     = New(40400, 404, "资源不存在")
	ErrConflict     = New(40900, 409, "数据冲突")
	ErrInternal     = New(50000, 500, "系统内部错误")
)
