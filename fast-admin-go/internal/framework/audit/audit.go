// Package audit 把"当前操作人"放进 context.Context，供 GORM 的 BeforeCreate/BeforeUpdate
// 钩子读取来填充 created_by/updated_by，对应 Java 侧的 AuditContextHolder + AuditContextFilter。
package audit

import "context"

type ctxKey struct{}

type Actor struct {
	UserID   string
	Username string
}

const (
	systemUserID   = "NOT_LOGIN"
	systemUsername = "系统管理员"
)

// WithActor 把当前登录用户写入 context，业务代码在 handler 里从 Session 取出
// userID/username 后调用一次即可，后续所有 repo.WithContext(ctx) 都能读到。
func WithActor(ctx context.Context, userID, username string) context.Context {
	return context.WithValue(ctx, ctxKey{}, Actor{UserID: userID, Username: username})
}

// FromContext 取出当前操作人，未登录场景（比如登录接口本身）返回系统默认值，
// 语义对齐 Java 侧未登录时填充的 "NOT_LOGIN"/"系统管理员"。
func FromContext(ctx context.Context) (userID, username string) {
	if ctx == nil {
		return systemUserID, systemUsername
	}
	actor, ok := ctx.Value(ctxKey{}).(Actor)
	if !ok {
		return systemUserID, systemUsername
	}
	return actor.UserID, actor.Username
}
