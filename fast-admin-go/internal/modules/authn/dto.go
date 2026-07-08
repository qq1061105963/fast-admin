// Package authn 是登录认证模块（HTTP 路径仍是 /auth，包名用 authn 是为了跟
// framework/auth 包区分开，避免在依赖它的地方要写别名导入）。
package authn

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// Captcha/UUID/UID 对应 Java 侧 LoginDto 里定义了但当前登录逻辑并未校验的字段，
	// 原样保留字段位置以兼容前端已有的登录表单，不做任何校验。
	Captcha string `json:"captcha"`
	UUID    string `json:"uuid"`
	UID     string `json:"uid"`
}

type LoginResponse struct {
	AccessToken string `json:"accessToken"`
}
