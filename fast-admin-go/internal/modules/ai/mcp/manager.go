// Package mcp 管理外部 MCP 服务的连接生命周期，并把它们暴露的工具转成 agent 可调用的
// tool.Spec，对应 Java 侧的 AiMcpClientManager。基于官方 Go MCP SDK
// (github.com/modelcontextprotocol/go-sdk) 实现 stdio / sse / streamable-http 三种传输。
// 保活在 Go 侧用内部 ticker 实现（不落 sys_job），启用保活的 SSE 服务按各自间隔 ping。
package mcp

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/settings"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/tool"
)

const connectTimeout = 30 * time.Second

// Manager 维护 serverID -> 连接 的缓存，线程安全。
type Manager struct {
	svc *Service
	set *settings.Settings

	mu         sync.RWMutex
	conns      map[string]*conn
	statuses   map[string]serverStatus
	keepalives map[string]context.CancelFunc
}

type conn struct {
	session *mcp.ClientSession
	snap    *snapshot
}

type snapshot struct {
	instructions      string
	serverInfo        map[string]any
	capabilities      map[string]any
	tools             []ToolInfo
	prompts           []PromptInfo
	resources         []ResourceInfo
	contextTokenCount int
}

type serverStatus struct {
	connected         bool
	toolCount         int
	promptCount       int
	resourceCount     int
	contextTokenCount int
	message           string
}

func NewManager(svc *Service, set *settings.Settings) *Manager {
	return &Manager{
		svc: svc, set: set,
		conns: map[string]*conn{}, statuses: map[string]serverStatus{}, keepalives: map[string]context.CancelFunc{},
	}
}

func (m *Manager) clientEnabled(ctx context.Context) bool { return m.set.MCPClientEnabled(ctx) }

// Reload 重连全部已启用服务。启动时可异步调用，单个连接失败只告警不阻塞。
func (m *Manager) Reload() {
	ctx := context.Background()
	m.closeAll()
	if !m.clientEnabled(ctx) {
		return
	}
	servers, err := m.svc.ListEnabled(ctx)
	if err != nil {
		logger.L().Sugar().Warnf("MCP listEnabled 失败: %v", err)
		return
	}
	for i := range servers {
		m.connect(ctx, &servers[i])
	}
}

// ReloadOne 只重连单个服务。
func (m *Manager) ReloadOne(id string) {
	ctx := context.Background()
	if !m.clientEnabled(ctx) {
		m.Remove(id)
		return
	}
	server, err := m.svc.GetByIDOrErr(ctx, id)
	if err != nil {
		return
	}
	m.closeOne(id)
	if !server.Enabled {
		m.setStatus(id, serverStatus{message: "未启用"})
		return
	}
	m.connect(ctx, server)
}

// Remove 关闭并移除单个服务连接与状态。
func (m *Manager) Remove(id string) {
	m.closeOne(id)
	m.mu.Lock()
	delete(m.statuses, id)
	m.mu.Unlock()
}

// connect 建立连接、拉取快照、登记状态与保活。
func (m *Manager) connect(ctx context.Context, server *AiMcpServer) {
	session, err := m.dial(server)
	if err != nil {
		m.setStatus(server.ID, serverStatus{message: err.Error()})
		logger.L().Sugar().Warnf("MCP 服务 '%s' 连接失败，跳过: %v", server.Name, err)
		return
	}
	snap := m.inspectSession(session)
	m.mu.Lock()
	m.conns[server.ID] = &conn{session: session, snap: snap}
	m.mu.Unlock()
	m.setStatus(server.ID, statusFromSnapshot(snap))
	m.startKeepalive(server)
	logger.L().Sugar().Infof("MCP 服务已连接: %s (%d tools)", server.Name, len(snap.tools))
}

// dial 按传输类型创建 SDK 客户端并连接。
func (m *Manager) dial(server *AiMcpServer) (*mcp.ClientSession, error) {
	transport, err := m.transport(server)
	if err != nil {
		return nil, err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "fast-admin", Version: "0.0.1"}, nil)
	connectCtx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return nil, errs.New(40140, 400, "MCP 连接失败："+err.Error())
	}
	return session, nil
}

func (m *Manager) transport(server *AiMcpServer) (mcp.Transport, error) {
	switch server.Transport {
	case "stdio":
		args := parseJSONArray(server.ArgsJSON)
		cmd := exec.Command(server.Command, args...)
		return &mcp.CommandTransport{Command: cmd}, nil
	case "sse":
		return &mcp.SSEClientTransport{Endpoint: server.URL, HTTPClient: httpClientWithHeaders(server.HeadersJSON)}, nil
	case "streamable-http":
		return &mcp.StreamableClientTransport{Endpoint: server.URL, HTTPClient: httpClientWithHeaders(server.HeadersJSON)}, nil
	default:
		return nil, errs.New(40141, 400, "不支持的 MCP 传输类型: "+server.Transport)
	}
}

// inspectSession 拉取 initialize 信息与工具/提示词/资源清单。
func (m *Manager) inspectSession(session *mcp.ClientSession) *snapshot {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	snap := &snapshot{}
	if init := session.InitializeResult(); init != nil {
		snap.instructions = init.Instructions
		snap.serverInfo = toMap(init.ServerInfo)
		snap.capabilities = toMap(init.Capabilities)
	}
	if res, err := session.ListTools(ctx, nil); err == nil {
		jsonInto(res.Tools, &snap.tools)
	}
	if res, err := session.ListPrompts(ctx, nil); err == nil {
		jsonInto(res.Prompts, &snap.prompts)
	}
	if res, err := session.ListResources(ctx, nil); err == nil {
		jsonInto(res.Resources, &snap.resources)
	}
	snap.contextTokenCount = estimateContextTokens(snap)
	return snap
}

// ApplyStatus 把运行时状态回填到 server 实体的 gorm:"-" 字段。
func (m *Manager) ApplyStatus(server *AiMcpServer) {
	if server == nil {
		return
	}
	set := func(connected bool, tc, pc, rc, ctc int, msg string) {
		b := connected
		server.Connected = &b
		server.ToolCount = &tc
		server.PromptCount = &pc
		server.ResourceCount = &rc
		server.ContextTokenCount = &ctc
		server.StatusMessage = msg
	}
	if !server.Enabled {
		set(false, 0, 0, 0, 0, "未启用")
		return
	}
	m.mu.RLock()
	status, ok := m.statuses[server.ID]
	m.mu.RUnlock()
	if !ok {
		set(false, 0, 0, 0, 0, "未加载")
		return
	}
	set(status.connected, status.toolCount, status.promptCount, status.resourceCount, status.contextTokenCount, status.message)
}

// Inspect 返回单个服务的运行时详情。
func (m *Manager) Inspect(ctx context.Context, id string) (*InspectDTO, error) {
	server, err := m.svc.GetByIDOrErr(ctx, id)
	if err != nil {
		return nil, err
	}
	m.ApplyStatus(server)
	m.mu.RLock()
	c, ok := m.conns[id]
	m.mu.RUnlock()
	if !ok || c == nil {
		return &InspectDTO{
			Server:  server,
			Runtime: RuntimeInfo{Connected: server.Connected, StatusMessage: server.StatusMessage, ToolCount: server.ToolCount, PromptCount: server.PromptCount, ResourceCount: server.ResourceCount, ContextTokenCount: server.ContextTokenCount, ServerInfo: map[string]any{}, Capabilities: map[string]any{}},
			Tools:   []ToolInfo{}, Prompts: []PromptInfo{}, Resources: []ResourceInfo{},
		}, nil
	}
	// 刷新一次快照
	snap := m.inspectSession(c.session)
	m.mu.Lock()
	c.snap = snap
	m.mu.Unlock()
	m.setStatus(id, statusFromSnapshot(snap))
	m.ApplyStatus(server)
	connected := true
	return &InspectDTO{
		Server: server,
		Runtime: RuntimeInfo{
			Connected: &connected, StatusMessage: "已连接",
			ToolCount: intPtr(len(snap.tools)), PromptCount: intPtr(len(snap.prompts)),
			ResourceCount: intPtr(len(snap.resources)), ContextTokenCount: intPtr(snap.contextTokenCount),
			Instructions: snap.instructions, ServerInfo: nonNilMap(snap.serverInfo), Capabilities: nonNilMap(snap.capabilities),
		},
		Tools: nonNilTools(snap.tools), Prompts: nonNilPrompts(snap.prompts), Resources: nonNilResources(snap.resources),
	}, nil
}

// ListSpecs 返回选定（serverIDs 为空则全部）已连接服务的工具 Spec，供 agent 挂载。
func (m *Manager) ListSpecs(ctx context.Context, serverIDs []string) []tool.Spec {
	if !m.clientEnabled(ctx) {
		return nil
	}
	selected := map[string]bool{}
	for _, id := range serverIDs {
		if strings.TrimSpace(id) != "" {
			selected[id] = true
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var specs []tool.Spec
	for id, c := range m.conns {
		if len(selected) > 0 && !selected[id] {
			continue
		}
		session := c.session
		for _, ti := range c.snap.tools {
			t := ti
			specs = append(specs, tool.Spec{
				Name: t.Name, Description: descOr(t.Description, t.Name), InputSchema: nonNilMap(t.InputSchema), Source: "mcp",
				Call: func(ctx context.Context, args map[string]any) (string, error) {
					return callMCPTool(ctx, session, t.Name, args)
				},
			})
		}
	}
	return specs
}

// PingServer 对活动连接发一次 ping，失败则单服重连；供保活 ticker 调用。
func (m *Manager) PingServer(id string) {
	ctx := context.Background()
	if !m.clientEnabled(ctx) || strings.TrimSpace(id) == "" {
		return
	}
	m.mu.RLock()
	c, ok := m.conns[id]
	m.mu.RUnlock()
	if !ok || c == nil {
		m.ReloadOne(id)
		return
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := c.session.Ping(pingCtx, nil); err != nil {
		logger.L().Sugar().Warnf("MCP 保活 ping 失败，重连服务 %s: %v", id, err)
		m.ReloadOne(id)
	}
}

// Close 关闭全部连接，供优雅退出调用。
func (m *Manager) Close() { m.closeAll() }

func callMCPTool(ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	res, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", errs.New(40142, 400, "MCP 工具调用失败："+err.Error())
	}
	var sb strings.Builder
	for _, content := range res.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	if sb.Len() > 0 {
		return sb.String(), nil
	}
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		return string(b), nil
	}
	b, _ := json.Marshal(res)
	return string(b), nil
}

// ---- 保活 ----

func (m *Manager) startKeepalive(server *AiMcpServer) {
	if server.Transport != "sse" || !server.KeepAlive {
		return
	}
	interval := server.KeepAliveInterval
	if interval < minKeepAliveSeconds {
		interval = defaultKeepAliveSeconds
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	if old, ok := m.keepalives[server.ID]; ok {
		old()
	}
	m.keepalives[server.ID] = cancel
	m.mu.Unlock()

	id := server.ID
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.PingServer(id)
			}
		}
	}()
}

func (m *Manager) stopKeepalive(id string) {
	if cancel, ok := m.keepalives[id]; ok {
		cancel()
		delete(m.keepalives, id)
	}
}

// ---- 状态 / 连接维护 ----

func (m *Manager) setStatus(id string, status serverStatus) {
	m.mu.Lock()
	m.statuses[id] = status
	m.mu.Unlock()
}

func (m *Manager) closeOne(id string) {
	m.mu.Lock()
	m.stopKeepalive(id)
	c := m.conns[id]
	delete(m.conns, id)
	m.mu.Unlock()
	if c != nil && c.session != nil {
		_ = c.session.Close()
	}
}

func (m *Manager) closeAll() {
	m.mu.Lock()
	for id, cancel := range m.keepalives {
		cancel()
		delete(m.keepalives, id)
	}
	conns := m.conns
	m.conns = map[string]*conn{}
	m.mu.Unlock()
	for _, c := range conns {
		if c.session != nil {
			_ = c.session.Close()
		}
	}
}

// ---- 工具函数 ----

func statusFromSnapshot(snap *snapshot) serverStatus {
	return serverStatus{
		connected: true, toolCount: len(snap.tools), promptCount: len(snap.prompts),
		resourceCount: len(snap.resources), contextTokenCount: snap.contextTokenCount, message: "已连接",
	}
}

func httpClientWithHeaders(headersJSON string) *http.Client {
	headers := parseJSONObject(headersJSON)
	if len(headers) == 0 {
		return nil
	}
	return &http.Client{Transport: &headerRoundTripper{headers: headers, base: http.DefaultTransport}}
}

type headerRoundTripper struct {
	headers map[string]string
	base    http.RoundTripper
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	return h.base.RoundTrip(req)
}

func parseJSONArray(jsonStr string) []string {
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}
	var arr []any
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		out = append(out, toStrVal(v))
	}
	return out
}

func parseJSONObject(jsonStr string) map[string]string {
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = toStrVal(v)
	}
	return out
}

func estimateContextTokens(snap *snapshot) int {
	payload := map[string]any{
		"instructions": snap.instructions, "tools": snap.tools, "prompts": snap.prompts, "resources": snap.resources,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	n := int(math.Ceil(float64(len(b)) / 4.0))
	if n < 1 {
		return 1
	}
	return n
}

func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{}
	}
	return m
}

func jsonInto(src, dst any) {
	b, err := json.Marshal(src)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, dst)
}

func toStrVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func descOr(desc, fallback string) string {
	if strings.TrimSpace(desc) == "" {
		return fallback
	}
	return desc
}

func intPtr(v int) *int { return &v }

func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func nonNilTools(v []ToolInfo) []ToolInfo {
	if v == nil {
		return []ToolInfo{}
	}
	return v
}

func nonNilPrompts(v []PromptInfo) []PromptInfo {
	if v == nil {
		return []PromptInfo{}
	}
	return v
}

func nonNilResources(v []ResourceInfo) []ResourceInfo {
	if v == nil {
		return []ResourceInfo{}
	}
	return v
}
