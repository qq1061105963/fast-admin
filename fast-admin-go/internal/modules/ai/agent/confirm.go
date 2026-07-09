package agent

import (
	"sync"
	"time"
)

const confirmTimeout = 120 * time.Second

// ConfirmationService 管理需要用户二次确认的工具调用（execute_sql）。
// 工具执行前 WaitForConfirmation 阻塞，前端收到 pending 事件后回调 Respond 唤醒。
type ConfirmationService struct {
	mu      sync.Mutex
	pending map[string]chan bool
}

func NewConfirmationService() *ConfirmationService {
	return &ConfirmationService{pending: map[string]chan bool{}}
}

// WaitForConfirmation 阻塞等待用户确认，超时或取消返回 false。
func (c *ConfirmationService) WaitForConfirmation(token string) bool {
	ch := make(chan bool, 1)
	c.mu.Lock()
	c.pending[token] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, token)
		c.mu.Unlock()
	}()
	select {
	case v := <-ch:
		return v
	case <-time.After(confirmTimeout):
		return false
	}
}

// Respond 唤醒等待中的确认；token 不存在返回 false。
func (c *ConfirmationService) Respond(token string, confirmed bool) bool {
	c.mu.Lock()
	ch, ok := c.pending[token]
	if ok {
		delete(c.pending, token)
	}
	c.mu.Unlock()
	if !ok {
		return false
	}
	ch <- confirmed
	return true
}
