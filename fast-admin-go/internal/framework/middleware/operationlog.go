package middleware

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/oplog"
)

// bodyCaptureWriter 包一层 gin.ResponseWriter，把写出去的响应体同时缓冲一份，
// 用于操作日志记录（Go 没有像 Java 那样能在 AOP 里直接拿到方法返回值，
// 只能在 HTTP 层截获响应字节）。
type bodyCaptureWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyCaptureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.Hijack()
}

// OperationLog 是 @OperationLog 注解 + OperationLogAspect 的中间件版本：
// 记录请求参数、响应结果、耗时、操作人，脱敏后异步落库。挂在具体路由上使用，
// title/bizType 对应注解的两个核心参数。
func OperationLog(writer oplog.Writer, title string, bizType oplog.BusinessType) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		var reqBody []byte
		if c.Request.Body != nil {
			reqBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		writerWrap := &bodyCaptureWriter{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = writerWrap

		c.Next()

		status := int8(1)
		errorMsg := ""
		if len(c.Errors) > 0 {
			status = 0
			errorMsg = oplog.MaskSensitive(c.Errors.String())
		} else if c.Writer.Status() >= http.StatusBadRequest {
			status = 0
		}

		userID, username := "", ""
		if session, ok := CurrentSession(c); ok {
			userID, username = session.UserID, session.Username
		}

		entry := oplog.Entry{
			Title:          title,
			BusinessType:   bizType,
			Method:         c.HandlerName(),
			RequestMethod:  c.Request.Method,
			OperatorType:   "ADMIN",
			UserID:         userID,
			Username:       username,
			URL:            c.Request.URL.Path,
			IP:             c.ClientIP(),
			RequestParams:  oplog.MaskSensitive(mergeRequestParams(c, reqBody)),
			ResponseResult: oplog.MaskSensitive(oplog.TruncateResponse(writerWrap.buf.String())),
			Status:         status,
			ErrorMsg:       errorMsg,
			CostTime:       time.Since(start).Milliseconds(),
			CreatedAt:      start,
		}
		writer.Save(entry)
	}
}

func mergeRequestParams(c *gin.Context, body []byte) string {
	if len(body) > 0 {
		return string(body)
	}
	if q := c.Request.URL.RawQuery; q != "" {
		return q
	}
	return ""
}
