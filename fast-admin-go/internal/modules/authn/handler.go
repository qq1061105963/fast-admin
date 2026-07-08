package authn

import (
	"github.com/gin-gonic/gin"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/response"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/useragent"
)

type Handler struct {
	svc         *Service
	tokenHeader string
}

func NewHandler(svc *Service, tokenHeader string) *Handler {
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}
	return &Handler{svc: svc, tokenHeader: tokenHeader}
}

func loginContext(c *gin.Context) LoginContext {
	ua := c.Request.UserAgent()
	return LoginContext{
		IP: c.ClientIP(), UserAgent: ua,
		Browser: useragent.ParseBrowser(ua), OS: useragent.ParseOS(ua), Device: useragent.ParseDevice(ua),
	}
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.ErrBadRequest.Wrap(err))
		return
	}
	token, err := h.svc.Login(c.Request.Context(), &req, loginContext(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, LoginResponse{AccessToken: token})
}

func (h *Handler) Logout(c *gin.Context) {
	token := c.GetHeader(h.tokenHeader)
	h.svc.Logout(c.Request.Context(), token, loginContext(c))
	response.Success(c, nil)
}

// Codes 直接从鉴权中间件写入的 Session 里取权限码，语义等价于 Java 侧重新查库，
// 因为登录时已经把权限码算好放进了 Session。
func (h *Handler) Codes(c *gin.Context) {
	session, ok := middleware.CurrentSession(c)
	if !ok {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	response.Success(c, session.Permissions)
}
