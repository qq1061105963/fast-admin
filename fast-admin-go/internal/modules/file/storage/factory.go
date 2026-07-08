package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ParseConfig 把 fileconfig 表里存的 JSON 反序列化成具体的 Config 实现。
func ParseConfig(typ Type, raw []byte) (Config, error) {
	switch typ {
	case TypeLocal:
		var c LocalConfig
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		return &c, nil
	case TypeOSS:
		var c OssConfig
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		return &c, nil
	case TypeS3:
		var c S3Config
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		return &c, nil
	case TypeSFTP:
		var c SftpConfig
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		return &c, nil
	case TypeFTP:
		var c FtpConfig
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		return &c, nil
	default:
		return nil, fmt.Errorf("storage: unsupported type %q", typ)
	}
}

// NewRegistry 注册全部内置存储实现，对应 Java 侧 Spring 自动收集所有 FileStorage Bean。
func NewRegistry() map[Type]Storage {
	return map[Type]Storage{
		TypeLocal: NewLocalStorage(),
		TypeOSS:   NewOssStorage(),
		TypeS3:    NewS3Storage(),
		TypeSFTP:  NewSftpStorage(),
		TypeFTP:   NewFtpStorage(),
	}
}

// ActiveConfigProvider 由 fileconfig 模块实现，Factory 借此查询"当前激活配置"，
// 避免 storage 包反向依赖 fileconfig 包（依赖方向：fileconfig -> storage）。
type ActiveConfigProvider interface {
	ActiveConfig(ctx context.Context) (configID string, typ Type, rawConfig []byte, urlPrefix string, err error)
}

type Active struct {
	ConfigID  string
	URLPrefix string
	Config    Config
	Storage   Storage
}

// Factory 懒加载 + 缓存当前激活配置，对应 Java 侧 FileStorageFactory 的
// "进程内单例缓存 + 事件驱动失效"设计；这里 Invalidate() 就是那个失效入口，
// 由 fileconfig.Service 在配置变更/激活切换时调用。
type Factory struct {
	mu       sync.Mutex
	registry map[Type]Storage
	provider ActiveConfigProvider
	active   *Active
}

// NewFactory 只接收 registry；provider 通常要等 fileconfig.Service 构造完成后
// 才能拿到（fileconfig.Service 自己又需要持有 Factory 来触发缓存失效），
// 用 SetProvider 做两阶段初始化打破这个构造顺序循环。
func NewFactory(registry map[Type]Storage) *Factory {
	return &Factory{registry: registry}
}

func (f *Factory) SetProvider(provider ActiveConfigProvider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.provider = provider
}

func (f *Factory) Invalidate() {
	f.mu.Lock()
	f.active = nil
	f.mu.Unlock()
}

func (f *Factory) Current(ctx context.Context) (*Active, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active != nil {
		return f.active, nil
	}
	if f.provider == nil {
		return nil, fmt.Errorf("storage: factory has no active config provider wired yet")
	}
	id, typ, raw, urlPrefix, err := f.provider.ActiveConfig(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := ParseConfig(typ, raw)
	if err != nil {
		return nil, err
	}
	st, ok := f.registry[typ]
	if !ok {
		return nil, fmt.Errorf("storage: unsupported type %q", typ)
	}
	f.active = &Active{ConfigID: id, URLPrefix: urlPrefix, Config: cfg, Storage: st}
	return f.active, nil
}

// Resolve 按指定的类型 + 原始 JSON 配置直接构造一个 Storage 调用上下文，
// 不经过缓存，供文件下载/删除时按"上传时的历史配置"访问用。
func (f *Factory) Resolve(typ Type, raw []byte) (Storage, Config, error) {
	cfg, err := ParseConfig(typ, raw)
	if err != nil {
		return nil, nil, err
	}
	st, ok := f.registry[typ]
	if !ok {
		return nil, nil, fmt.Errorf("storage: unsupported type %q", typ)
	}
	return st, cfg, nil
}
