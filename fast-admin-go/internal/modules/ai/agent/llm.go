package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

// 本文件手写 Anthropic Messages / OpenAI Chat Completions 的流式 + 工具调用循环，
// 取代 Java 侧 Spring AI ChatClient 自动完成的 agent loop：调模型 → 若请求工具则执行
// 并把结果回填 → 再调，直到模型给出最终回答或达到最大工具轮次。

type modelConfig struct {
	Provider    string
	Model       string
	APIKey      string
	BaseURL     string
	Temperature *float64
	MaxTokens   *int
}

// chatMsg 是注入模型的历史消息（纯文本）。
type chatMsg struct {
	Role    string // user / assistant
	Content string
}

type tokenUsage struct {
	Prompt     *int
	Completion *int
	Total      *int
}

type toolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// toolExec 执行一次工具调用，返回结果文本与是否出错（出错也要回填给模型让其继续）。
type toolExec func(ctx context.Context, name string, args map[string]any) (result string, isError bool)

const anthropicDefaultMaxTokens = 4096

// streamChat 按 provider 分发到对应的流式循环。
func streamChat(ctx context.Context, cfg modelConfig, system string, history []chatMsg, userMsg string,
	tools []toolDef, maxIters int, emit func(delta string), exec toolExec) (string, tokenUsage, error) {
	if cfg.Provider == "anthropic" {
		return streamAnthropic(ctx, cfg, system, history, userMsg, tools, maxIters, emit, exec)
	}
	return streamOpenAI(ctx, cfg, system, history, userMsg, tools, maxIters, emit, exec)
}

// ---- OpenAI ----

func streamOpenAI(ctx context.Context, cfg modelConfig, system string, history []chatMsg, userMsg string,
	tools []toolDef, maxIters int, emit func(delta string), exec toolExec) (string, tokenUsage, error) {
	base := normalizeOpenAIBase(cfg.BaseURL)
	messages := []map[string]any{{"role": "system", "content": system}}
	for _, h := range history {
		messages = append(messages, map[string]any{"role": h.Role, "content": h.Content})
	}
	messages = append(messages, map[string]any{"role": "user", "content": userMsg})

	var usage tokenUsage
	var finalAnswer string
	for iter := 0; iter <= maxIters; iter++ {
		lastRound := iter == maxIters
		body := map[string]any{
			"model": cfg.Model, "messages": messages, "stream": true,
			"stream_options": map[string]any{"include_usage": true},
		}
		if cfg.Temperature != nil {
			body["temperature"] = *cfg.Temperature
		}
		if cfg.MaxTokens != nil {
			body["max_tokens"] = *cfg.MaxTokens
		}
		if len(tools) > 0 && !lastRound {
			body["tools"] = openAITools(tools)
			body["tool_choice"] = "auto"
		}
		resp, err := doStreamPost(ctx, base+"/chat/completions", map[string]string{
			"Authorization": "Bearer " + cfg.APIKey,
		}, body)
		if err != nil {
			return finalAnswer, usage, err
		}

		content, calls, u, err := readOpenAIStream(resp, emit)
		if err != nil {
			return finalAnswer, usage, err
		}
		if u != nil {
			usage = *u
		}
		finalAnswer = content
		if len(calls) == 0 {
			return finalAnswer, usage, nil
		}
		// 追加 assistant 的 tool_calls 消息 + 各工具结果消息
		toolCalls := make([]map[string]any, 0, len(calls))
		for _, c := range calls {
			toolCalls = append(toolCalls, map[string]any{
				"id": c.id, "type": "function",
				"function": map[string]any{"name": c.name, "arguments": c.args},
			})
		}
		messages = append(messages, map[string]any{"role": "assistant", "content": content, "tool_calls": toolCalls})
		for _, c := range calls {
			result := runToolCall(ctx, exec, c.name, c.args)
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": c.id, "content": result})
		}
	}
	return finalAnswer, usage, nil
}

type openAIToolCall struct {
	id   string
	name string
	args string
}

func readOpenAIStream(resp *http.Response, emit func(string)) (string, []openAIToolCall, *tokenUsage, error) {
	defer resp.Body.Close()
	var content strings.Builder
	callsByIndex := map[int]*openAIToolCall{}
	var order []int
	var usage *tokenUsage

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			data := strings.TrimSpace(line)
			if strings.HasPrefix(data, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(data, "data:"))
				if payload == "[DONE]" {
					break
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content   string `json:"content"`
							ToolCalls []struct {
								Index    int    `json:"index"`
								ID       string `json:"id"`
								Function struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								} `json:"function"`
							} `json:"tool_calls"`
						} `json:"delta"`
					} `json:"choices"`
					Usage *struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
						TotalTokens      int `json:"total_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal([]byte(payload), &chunk) == nil {
					for _, ch := range chunk.Choices {
						if ch.Delta.Content != "" {
							content.WriteString(ch.Delta.Content)
							emit(ch.Delta.Content)
						}
						for _, tc := range ch.Delta.ToolCalls {
							call, ok := callsByIndex[tc.Index]
							if !ok {
								call = &openAIToolCall{}
								callsByIndex[tc.Index] = call
								order = append(order, tc.Index)
							}
							if tc.ID != "" {
								call.id = tc.ID
							}
							if tc.Function.Name != "" {
								call.name = tc.Function.Name
							}
							call.args += tc.Function.Arguments
						}
					}
					if chunk.Usage != nil {
						usage = &tokenUsage{
							Prompt: intPtr(chunk.Usage.PromptTokens), Completion: intPtr(chunk.Usage.CompletionTokens),
							Total: intPtr(chunk.Usage.TotalTokens),
						}
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	calls := make([]openAIToolCall, 0, len(order))
	for _, idx := range order {
		calls = append(calls, *callsByIndex[idx])
	}
	return content.String(), calls, usage, nil
}

func openAITools(tools []toolDef) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": t.Name, "description": t.Description, "parameters": t.InputSchema,
			},
		})
	}
	return out
}

// ---- Anthropic ----

func streamAnthropic(ctx context.Context, cfg modelConfig, system string, history []chatMsg, userMsg string,
	tools []toolDef, maxIters int, emit func(delta string), exec toolExec) (string, tokenUsage, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	maxTokens := anthropicDefaultMaxTokens
	if cfg.MaxTokens != nil {
		maxTokens = *cfg.MaxTokens
	}

	var messages []map[string]any
	for _, h := range history {
		messages = append(messages, map[string]any{"role": h.Role, "content": h.Content})
	}
	messages = append(messages, map[string]any{"role": "user", "content": userMsg})

	var usage tokenUsage
	var finalAnswer string
	for iter := 0; iter <= maxIters; iter++ {
		lastRound := iter == maxIters
		body := map[string]any{
			"model": cfg.Model, "system": system, "messages": messages, "stream": true, "max_tokens": maxTokens,
		}
		if cfg.Temperature != nil {
			body["temperature"] = *cfg.Temperature
		}
		if len(tools) > 0 && !lastRound {
			body["tools"] = anthropicTools(tools)
		}
		resp, err := doStreamPost(ctx, base+"/v1/messages", map[string]string{
			"x-api-key": cfg.APIKey, "anthropic-version": "2023-06-01",
		}, body)
		if err != nil {
			return finalAnswer, usage, err
		}
		text, toolUses, stopReason, u, err := readAnthropicStream(resp, emit)
		if err != nil {
			return finalAnswer, usage, err
		}
		if u != nil {
			usage = mergeUsage(usage, *u)
		}
		finalAnswer = text
		if stopReason != "tool_use" || len(toolUses) == 0 {
			return finalAnswer, usage, nil
		}
		// assistant 回合内容块（文本 + tool_use）
		assistantContent := []map[string]any{}
		if strings.TrimSpace(text) != "" {
			assistantContent = append(assistantContent, map[string]any{"type": "text", "text": text})
		}
		for _, tu := range toolUses {
			assistantContent = append(assistantContent, map[string]any{
				"type": "tool_use", "id": tu.id, "name": tu.name, "input": tu.input(),
			})
		}
		messages = append(messages, map[string]any{"role": "assistant", "content": assistantContent})
		// user 回合回填 tool_result
		toolResults := make([]map[string]any, 0, len(toolUses))
		for _, tu := range toolUses {
			args := tu.input()
			result := runToolCallMap(ctx, exec, tu.name, args)
			toolResults = append(toolResults, map[string]any{
				"type": "tool_result", "tool_use_id": tu.id, "content": result,
			})
		}
		messages = append(messages, map[string]any{"role": "user", "content": toolResults})
	}
	return finalAnswer, usage, nil
}

type anthropicToolUse struct {
	id      string
	name    string
	jsonBuf string
}

func (t *anthropicToolUse) input() map[string]any {
	if strings.TrimSpace(t.jsonBuf) == "" {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal([]byte(t.jsonBuf), &m) != nil {
		return map[string]any{}
	}
	return m
}

func readAnthropicStream(resp *http.Response, emit func(string)) (string, []*anthropicToolUse, string, *tokenUsage, error) {
	defer resp.Body.Close()
	var text strings.Builder
	blocks := map[int]*anthropicToolUse{}
	var order []int
	stopReason := ""
	usage := &tokenUsage{}
	hasUsage := false

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			data := strings.TrimSpace(line)
			if strings.HasPrefix(data, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(data, "data:"))
				var evt struct {
					Type         string `json:"type"`
					Index        int    `json:"index"`
					ContentBlock *struct {
						Type string `json:"type"`
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"content_block"`
					Delta *struct {
						Type        string `json:"type"`
						Text        string `json:"text"`
						PartialJSON string `json:"partial_json"`
						StopReason  string `json:"stop_reason"`
					} `json:"delta"`
					Message *struct {
						Usage *struct {
							InputTokens  int `json:"input_tokens"`
							OutputTokens int `json:"output_tokens"`
						} `json:"usage"`
					} `json:"message"`
					Usage *struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal([]byte(payload), &evt) == nil {
					switch evt.Type {
					case "content_block_start":
						if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
							blocks[evt.Index] = &anthropicToolUse{id: evt.ContentBlock.ID, name: evt.ContentBlock.Name}
							order = append(order, evt.Index)
						}
					case "content_block_delta":
						if evt.Delta != nil {
							if evt.Delta.Type == "text_delta" && evt.Delta.Text != "" {
								text.WriteString(evt.Delta.Text)
								emit(evt.Delta.Text)
							}
							if evt.Delta.Type == "input_json_delta" {
								if b, ok := blocks[evt.Index]; ok {
									b.jsonBuf += evt.Delta.PartialJSON
								}
							}
						}
					case "message_start":
						if evt.Message != nil && evt.Message.Usage != nil {
							usage.Prompt = intPtr(evt.Message.Usage.InputTokens)
							hasUsage = true
						}
					case "message_delta":
						if evt.Delta != nil && evt.Delta.StopReason != "" {
							stopReason = evt.Delta.StopReason
						}
						if evt.Usage != nil {
							usage.Completion = intPtr(evt.Usage.OutputTokens)
							hasUsage = true
						}
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	toolUses := make([]*anthropicToolUse, 0, len(order))
	for _, idx := range order {
		toolUses = append(toolUses, blocks[idx])
	}
	if hasUsage {
		p, c := 0, 0
		if usage.Prompt != nil {
			p = *usage.Prompt
		}
		if usage.Completion != nil {
			c = *usage.Completion
		}
		usage.Total = intPtr(p + c)
		return text.String(), toolUses, stopReason, usage, nil
	}
	return text.String(), toolUses, stopReason, nil, nil
}

func anthropicTools(tools []toolDef) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		out = append(out, map[string]any{
			"name": t.Name, "description": t.Description, "input_schema": t.InputSchema,
		})
	}
	return out
}

// ---- 共用 ----

func runToolCall(ctx context.Context, exec toolExec, name, argsJSON string) string {
	var args map[string]any
	if strings.TrimSpace(argsJSON) != "" {
		_ = json.Unmarshal([]byte(argsJSON), &args)
	}
	if args == nil {
		args = map[string]any{}
	}
	return runToolCallMap(ctx, exec, name, args)
}

func runToolCallMap(ctx context.Context, exec toolExec, name string, args map[string]any) string {
	result, _ := exec(ctx, name, args)
	return result
}

func doStreamPost(ctx context.Context, url string, headers map[string]string, body map[string]any) (*http.Response, error) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, errs.New(40150, 400, "构建模型请求失败："+err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errs.New(40151, 400, "AI 响应失败："+err.Error())
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, errs.New(40152, 400, fmt.Sprintf("AI 响应失败（HTTP %d）：%s", resp.StatusCode, snippetN(raw, 500)))
	}
	return resp, nil
}

func normalizeOpenAIBase(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	if !strings.HasSuffix(base, "/v1") {
		base = base + "/v1"
	}
	return base
}

func mergeUsage(prev, next tokenUsage) tokenUsage {
	// 多轮工具调用会累计多次 usage，这里覆盖为最近一次（与 Java 取最后一帧一致）
	return next
}

func intPtr(v int) *int { return &v }

func snippetN(raw []byte, n int) string {
	s := string(raw)
	if len(s) > n {
		return s[:n]
	}
	return s
}
