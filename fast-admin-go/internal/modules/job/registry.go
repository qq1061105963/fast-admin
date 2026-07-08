package job

import (
	"context"
	"sync"
)

// JobFunc 是任务的实际执行体，替代 Java 侧"反射调用 Spring Bean 方法"的机制：
// beanName 直接映射到一个注册好的函数，methodParams 原样传入，函数自己决定怎么解析。
type JobFunc func(ctx context.Context, params string) error

// Registry 是 beanName -> JobFunc 的注册表，业务代码在启动时调用 Register
// 把可被定时任务调用的函数登记进来。
type Registry struct {
	mu  sync.RWMutex
	fns map[string]JobFunc
}

func NewRegistry() *Registry {
	return &Registry{fns: make(map[string]JobFunc)}
}

func (r *Registry) Register(beanName string, fn JobFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fns[beanName] = fn
}

func (r *Registry) Get(beanName string) (JobFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.fns[beanName]
	return fn, ok
}
