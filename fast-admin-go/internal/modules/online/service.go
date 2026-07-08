// Package online 实现在线用户列表和强制下线，完全基于 framework/auth.TokenService
// 维护的 Redis 会话数据，没有独立的数据库表（对应 Java 侧基于 Sa-Token session 的设计）。
package online

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/auth"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

type Dto struct {
	Token        string    `json:"token"`
	UserID       string    `json:"userId"`
	Username     string    `json:"username"`
	Nickname     string    `json:"nickname"`
	LoginIP      string    `json:"loginIp"`
	Browser      string    `json:"browser"`
	OS           string    `json:"os"`
	LoginTime    time.Time `json:"loginTime"`
	TokenTimeout int64     `json:"tokenTimeout"` // 剩余存活秒数
}

type Service struct {
	tokenSvc *auth.TokenService
}

func NewService(tokenSvc *auth.TokenService) *Service {
	return &Service{tokenSvc: tokenSvc}
}

func (s *Service) List(ctx context.Context, keyword string) ([]Dto, error) {
	sessions, err := s.tokenSvc.ListOnline(ctx)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	dtos := make([]Dto, 0, len(sessions))
	for _, os := range sessions {
		if keyword != "" &&
			!strings.Contains(strings.ToLower(os.Session.Username), keyword) &&
			!strings.Contains(strings.ToLower(os.Session.Nickname), keyword) {
			continue
		}
		dtos = append(dtos, Dto{
			Token: os.Token, UserID: os.Session.UserID, Username: os.Session.Username,
			Nickname: os.Session.Nickname, LoginIP: os.Session.IP, Browser: os.Session.Browser,
			OS: os.Session.OS, LoginTime: os.Session.LoginTime, TokenTimeout: int64(os.TTL.Seconds()),
		})
	}
	sort.SliceStable(dtos, func(i, j int) bool { return dtos[i].LoginTime.After(dtos[j].LoginTime) })
	return dtos, nil
}

func (s *Service) Kickout(ctx context.Context, token string) error {
	if err := s.tokenSvc.KickoutToken(ctx, token); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
