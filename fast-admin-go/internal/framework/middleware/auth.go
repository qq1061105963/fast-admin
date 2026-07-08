package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	authpkg "github.com/SirYuxuan/fast-admin-go/internal/framework/auth"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/audit"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
)

const sessionContextKey = "auth.session"

// Auth 校验请求头里的 token，成功后把 Session 塞进 gin.Context，
// 后续 handler 通过 CurrentSession(c) 取用。
func Auth(tokenService *authpkg.TokenService, tokenHeader string) gin.HandlerFunc {
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}
	return func(c *gin.Context) {
		token := c.GetHeader(tokenHeader)
		if token == "" {
			response.Fail(c, errs.ErrUnauthorized)
			c.Abort()
			return
		}

		session, err := tokenService.Parse(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				response.Fail(c, errs.ErrUnauthorized)
			} else {
				response.Fail(c, errs.ErrInternal.Wrap(err))
			}
			c.Abort()
			return
		}

		c.Set(sessionContextKey, session)
		// 把当前登录用户写入 request context，供 model.BaseModel 的
		// BeforeCreate/BeforeUpdate 钩子读取来填充 created_by/updated_by；
		// 漏了这一步的话所有审计字段会一直是未登录态的默认值。
		ctx := audit.WithActor(c.Request.Context(), session.UserID, session.Username)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequirePermission 要求当前登录用户拥有指定权限码，需放在 Auth 之后使用。
func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := CurrentSession(c)
		if !ok || !session.HasPermission(code) {
			response.Fail(c, errs.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// CurrentSession 从 gin.Context 里取出 Auth 中间件写入的会话信息。
func CurrentSession(c *gin.Context) (*authpkg.Session, bool) {
	v, ok := c.Get(sessionContextKey)
	if !ok {
		return nil, false
	}
	session, ok := v.(*authpkg.Session)
	return session, ok
}
