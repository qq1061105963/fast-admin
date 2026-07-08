// Package auth 实现一个轻量的 token/session 鉴权体系，替代 Java 侧的 Sa-Token。
// 语义对齐现有系统的 Sa-Token 配置：is-concurrent=true（同一账号允许多端同时在线）、
// is-share=false（每次登录签发新 token）、token-name=Authorization、30 天滑动过期。
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/config"
)

// Session 是登录成功后写入 Redis 的会话信息，同时供鉴权中间件和"在线用户"功能使用。
type Session struct {
	UserID      string    `json:"userId"`
	Username    string    `json:"username"`
	Nickname    string    `json:"nickname"`
	DeptID      string    `json:"deptId"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	Browser     string    `json:"browser"`
	OS          string    `json:"os"`
	IP          string    `json:"ip"`
	LoginTime   time.Time `json:"loginTime"`
}

// HasPermission 支持一个通配权限码 "*:*:*" 代表超级管理员。
func (s *Session) HasPermission(code string) bool {
	for _, p := range s.Permissions {
		if p == "*:*:*" || p == code {
			return true
		}
	}
	return false
}

// OnlineSession 是在线用户列表展示用的信息，多加了 token 本身和剩余存活时间。
type OnlineSession struct {
	Token   string
	Session Session
	TTL     time.Duration // 剩余存活时间，-1 表示永久（当前实现固定有过期时间，不会出现）
}

// TokenService 负责 token 的签发、校验、注销、强制下线、在线枚举。
type TokenService struct {
	rdb    *redis.Client
	ttl    time.Duration
	prefix string
}

func NewTokenService(rdb *redis.Client, cfg config.AuthConfig) *TokenService {
	ttl := time.Duration(cfg.TokenTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	prefix := cfg.TokenPrefix
	if prefix == "" {
		prefix = "auth"
	}
	return &TokenService{rdb: rdb, ttl: ttl, prefix: prefix}
}

func (s *TokenService) tokenKey(token string) string {
	return fmt.Sprintf("%s:token:%s", s.prefix, token)
}

func (s *TokenService) userTokensKey(userID string) string {
	return fmt.Sprintf("%s:user-tokens:%s", s.prefix, userID)
}

func (s *TokenService) allTokensKey() string {
	return fmt.Sprintf("%s:tokens:all", s.prefix)
}

// Login 签发一个新 token 并写入会话；不会顶掉同账号已有的其它 token（多端并发登录）。
func (s *TokenService) Login(ctx context.Context, session *Session) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	session.LoginTime = time.Now()

	payload, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.tokenKey(token), payload, s.ttl)
	pipe.SAdd(ctx, s.userTokensKey(session.UserID), token)
	pipe.Expire(ctx, s.userTokensKey(session.UserID), s.ttl)
	pipe.SAdd(ctx, s.allTokensKey(), token)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("persist session: %w", err)
	}
	return token, nil
}

// Parse 校验 token 并返回会话信息；token 无效或已过期返回 redis.Nil。
func (s *TokenService) Parse(ctx context.Context, token string) (*Session, error) {
	raw, err := s.rdb.Get(ctx, s.tokenKey(token)).Bytes()
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	s.rdb.Expire(ctx, s.tokenKey(token), s.ttl) // 滑动续期
	return &session, nil
}

// Logout 使指定 token 失效（只影响这一个端，不影响同账号其它设备）。
func (s *TokenService) Logout(ctx context.Context, token string) error {
	session, err := s.Parse(ctx, token)
	if err != nil {
		return nil // 已经失效，视为成功
	}
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, s.tokenKey(token))
	pipe.SRem(ctx, s.userTokensKey(session.UserID), token)
	pipe.SRem(ctx, s.allTokensKey(), token)
	_, err = pipe.Exec(ctx)
	return err
}

// KickoutUser 强制某个用户所有端下线，用于管理员在用户管理里禁用/删除账号时联动清理。
func (s *TokenService) KickoutUser(ctx context.Context, userID string) error {
	tokens, err := s.rdb.SMembers(ctx, s.userTokensKey(userID)).Result()
	if err != nil || len(tokens) == 0 {
		return nil
	}
	pipe := s.rdb.TxPipeline()
	for _, token := range tokens {
		pipe.Del(ctx, s.tokenKey(token))
		pipe.SRem(ctx, s.allTokensKey(), token)
	}
	pipe.Del(ctx, s.userTokensKey(userID))
	_, err = pipe.Exec(ctx)
	return err
}

// KickoutToken 按 token 精确强制下线一个会话，对应在线用户列表的"踢下线"操作。
func (s *TokenService) KickoutToken(ctx context.Context, token string) error {
	return s.Logout(ctx, token)
}

// TTL 返回 token 的剩余存活时间；-2 表示已不存在。
func (s *TokenService) TTL(ctx context.Context, token string) time.Duration {
	d, err := s.rdb.TTL(ctx, s.tokenKey(token)).Result()
	if err != nil {
		return -2 * time.Second
	}
	return d
}

// ListOnline 枚举所有在线会话。已过期的 token 会在这里被顺手从全局集合里清理掉
// （自愈式，不需要额外的定时任务）。
func (s *TokenService) ListOnline(ctx context.Context) ([]OnlineSession, error) {
	tokens, err := s.rdb.SMembers(ctx, s.allTokensKey()).Result()
	if err != nil {
		return nil, err
	}

	result := make([]OnlineSession, 0, len(tokens))
	staleTokens := make([]string, 0)
	for _, token := range tokens {
		raw, err := s.rdb.Get(ctx, s.tokenKey(token)).Bytes()
		if err != nil {
			staleTokens = append(staleTokens, token)
			continue
		}
		var session Session
		if err := json.Unmarshal(raw, &session); err != nil {
			staleTokens = append(staleTokens, token)
			continue
		}
		result = append(result, OnlineSession{Token: token, Session: session, TTL: s.TTL(ctx, token)})
	}
	if len(staleTokens) > 0 {
		s.rdb.SRem(ctx, s.allTokensKey(), toAny(staleTokens)...)
	}
	return result, nil
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
