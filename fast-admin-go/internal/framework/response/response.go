// Package response 提供统一的接口响应结构，对应 Java 侧的 Rs<T>/Ps<T>。
package response

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

// TraceIDKey 是 gin.Context 里存放链路追踪 ID 的 key，由 middleware.TraceID() 写入。
const TraceIDKey = "trace_id"

// Body 对应 Rs<T>：所有非分页接口的统一返回结构。
type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Timestamp int64  `json:"timestamp"`
	TraceID   string `json:"traceId"`
}

// PageData 对应 Ps<T> 的 data 字段形状：{total, items}。
type PageData struct {
	Total int64 `json:"total"`
	Items any   `json:"items"`
}

func traceID(c *gin.Context) string {
	v, _ := c.Get(TraceIDKey)
	id, _ := v.(string)
	return id
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:      0,
		Message:   "ok",
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
		TraceID:   traceID(c),
	})
}

// SuccessPage 返回 {total, items} 形状的分页数据，items 为 nil 时序列化成 []。
func SuccessPage(c *gin.Context, items any, total int64) {
	Success(c, PageData{Total: total, Items: items})
}

// Fail 将 error 转换为统一响应。若 err 是 *errs.AppError，使用其 Code/HTTPStatus/Message；
// 否则视为未预期的系统错误，统一按 500 处理，避免把内部错误细节泄漏给前端。
func Fail(c *gin.Context, err error) {
	var appErr *errs.AppError
	code, httpStatus, message := errs.ErrInternal.Code, errs.ErrInternal.HTTPStatus, errs.ErrInternal.Message
	if errors.As(err, &appErr) {
		code, httpStatus, message = appErr.Code, appErr.HTTPStatus, appErr.Message
	}
	c.JSON(httpStatus, Body{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
		TraceID:   traceID(c),
	})
}
