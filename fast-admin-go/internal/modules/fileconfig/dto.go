package fileconfig

import (
	"encoding/json"
	"strings"
	"time"
)

const maskedValue = "******"

var secretKeys = map[string]struct{}{
	"accesskey": {}, "secretkey": {}, "password": {}, "privatekey": {}, "passphrase": {},
}

func isSecretKey(key string) bool {
	_, ok := secretKeys[strings.ToLower(key)]
	return ok
}

// maskSecrets 把 config JSON 里的敏感字段值替换成 "******"，用于返回给前端。
func maskSecrets(raw string) map[string]any {
	m := map[string]any{}
	if raw == "" {
		return m
	}
	_ = json.Unmarshal([]byte(raw), &m)
	for k, v := range m {
		if isSecretKey(k) {
			if s, ok := v.(string); ok && s != "" {
				m[k] = maskedValue
			}
		}
	}
	return m
}

// mergeMaskedSecrets 把前端传回来的 config 和数据库里的旧 config 合并：
// 敏感字段如果前端传的是 "******"/空值，说明用户没有修改，沿用旧值；
// 否则采用前端传的新值。对应 Java 侧编辑时"密钥不回显、不改则不动"的规则。
func mergeMaskedSecrets(incoming map[string]any, oldRaw string) map[string]any {
	oldMap := map[string]any{}
	if oldRaw != "" {
		_ = json.Unmarshal([]byte(oldRaw), &oldMap)
	}
	merged := map[string]any{}
	for k, v := range incoming {
		merged[k] = v
	}
	for k := range incoming {
		if !isSecretKey(k) {
			continue
		}
		s, isStr := incoming[k].(string)
		if !isStr || s == "" || s == maskedValue {
			if old, ok := oldMap[k]; ok {
				merged[k] = old
			}
		}
	}
	return merged
}

type Dto struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	URLPrefix string         `json:"urlPrefix"`
	IsActive  bool           `json:"isActive"`
	Remark    string         `json:"remark"`
	CreatedAt time.Time      `json:"createdAt"`
}

func toDto(c Config) Dto {
	return Dto{
		ID: c.ID, Name: c.Name, Type: c.Type, Config: maskSecrets(c.RawConfig),
		URLPrefix: c.URLPrefix, IsActive: c.IsActive, Remark: c.Remark, CreatedAt: c.CreatedAt,
	}
}

type SaveRequest struct {
	ID        string         `json:"id"`
	Name      string         `json:"name" binding:"required,max=64"`
	Type      string         `json:"type" binding:"required"`
	Config    map[string]any `json:"config" binding:"required"`
	URLPrefix string         `json:"urlPrefix" binding:"required,max=255"`
	Remark    string         `json:"remark"`
}
