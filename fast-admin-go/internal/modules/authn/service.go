package authn

import (
	"context"
	"errors"
	"time"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/auth"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/loginlog"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/security"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/menu"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/permission"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/user"
	"gorm.io/gorm"
)

// LoginContext 携带跟 HTTP 请求相关的元信息，由 handler 从 gin.Context 提取后传入，
// 让 Service 保持不依赖 gin 包。
type LoginContext struct {
	IP        string
	UserAgent string
	Browser   string
	OS        string
	Device    string
}

type Service struct {
	userRepo *user.Repository
	permRepo *permission.Repository
	menuSvc  *menu.Service
	tokenSvc *auth.TokenService
	loginLog loginlog.Writer
}

func NewService(userRepo *user.Repository, permRepo *permission.Repository, menuSvc *menu.Service, tokenSvc *auth.TokenService, loginLog loginlog.Writer) *Service {
	return &Service{userRepo: userRepo, permRepo: permRepo, menuSvc: menuSvc, tokenSvc: tokenSvc, loginLog: loginLog}
}

const genericLoginError = "用户名或密码错误"

// Login 完整复刻 AuthService.login 的流程：查用户 -> BCrypt 校验密码 -> 校验状态
// -> 签发 token -> 写会话元信息 -> 记登录日志（成功/失败都记）。
func (s *Service) Login(ctx context.Context, req *LoginRequest, lc LoginContext) (string, error) {
	u, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		s.recordLogin(lc, "", req.Username, 0, genericLoginError)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errs.New(40101, 400, genericLoginError)
		}
		return "", errs.ErrInternal.Wrap(err)
	}

	if !security.VerifyPassword(req.Password, u.Password) {
		s.recordLogin(lc, u.ID, req.Username, 0, genericLoginError)
		return "", errs.New(40101, 400, genericLoginError)
	}

	if !u.IsStatusValid() {
		msg := u.StatusMessage()
		s.recordLogin(lc, u.ID, req.Username, 0, msg)
		return "", errs.New(40102, 400, msg)
	}

	roleIDs, err := s.permRepo.RoleIDsByUserID(ctx, u.ID)
	if err != nil {
		return "", errs.ErrInternal.Wrap(err)
	}
	codes, err := s.menuSvc.PermissionCodes(ctx, u.ID)
	if err != nil {
		return "", errs.ErrInternal.Wrap(err)
	}

	session := &auth.Session{
		UserID: u.ID, Username: u.Username, Nickname: u.Nickname, DeptID: u.DeptID,
		Roles: roleIDs, Permissions: codes,
		Browser: lc.Browser, OS: lc.OS, IP: lc.IP,
	}
	token, err := s.tokenSvc.Login(ctx, session)
	if err != nil {
		return "", errs.ErrInternal.Wrap(err)
	}

	_ = s.userRepo.UpdateLoginInfo(ctx, u.ID, lc.IP)
	s.recordLogin(lc, u.ID, req.Username, 1, "登录成功")
	return token, nil
}

// Logout 注销当前 token；即使未登录调用也不报错，对应 Java 侧的容错处理。
func (s *Service) Logout(ctx context.Context, token string, lc LoginContext) {
	username := ""
	userID := ""
	if session, err := s.tokenSvc.Parse(ctx, token); err == nil {
		username, userID = session.Username, session.UserID
	}
	_ = s.tokenSvc.Logout(ctx, token)
	s.record(lc, userID, username, 1, "登出成功", loginlog.TypeLogout)
}

func (s *Service) recordLogin(lc LoginContext, userID, username string, status int8, msg string) {
	s.record(lc, userID, username, status, msg, loginlog.TypeLogin)
}

func (s *Service) record(lc LoginContext, userID, username string, status int8, msg string, typ loginlog.Type) {
	if s.loginLog == nil {
		return
	}
	s.loginLog.Save(loginlog.Entry{
		UserID: userID, Username: username, IP: lc.IP, Browser: lc.Browser, OS: lc.OS,
		Device: lc.Device, Status: status, Msg: msg, Type: typ, CreatedAt: time.Now(),
	})
}
