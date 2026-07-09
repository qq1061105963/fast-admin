package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

// probe 通过 HTTP 直连模型服务，提供「获取模型列表」与「连通性测试」，不依赖任何 SDK，
// 便于保存前对链接/密钥做校验，对应 Java 侧的 AiModelProbeService。
const (
	anthropicDefaultBase = "https://api.anthropic.com"
	openAIDefaultBase    = "https://api.openai.com"
	anthropicVersion     = "2023-06-01"
	probeTimeout         = 20 * time.Second
	maxErrorChars        = 500
)

var probeClient = &http.Client{Timeout: probeTimeout}

// FetchModels 拉取服务方可用模型列表。
func FetchModels(ctx context.Context, provider, baseURL, apiKey string) ([]string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errs.New(40020, 400, "API Key 不能为空")
	}
	url, err := modelsURL(provider, baseURL)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	applyAuthHeaders(req, provider, apiKey)

	root, err := send(req, "获取模型列表")
	if err != nil {
		return nil, err
	}
	data, ok := root["data"].([]any)
	if !ok {
		return nil, errs.New(40021, 400, "模型列表返回格式不支持")
	}
	models := make([]string, 0, len(data))
	for _, item := range data {
		if m, ok := item.(map[string]any); ok {
			if id, ok := m["id"].(string); ok && id != "" {
				models = append(models, id)
			}
		}
	}
	return models, nil
}

// Test 发一次最小请求测连通性并返回延时（毫秒）。
func Test(ctx context.Context, provider, baseURL, apiKey, modelName string) (int64, error) {
	if strings.TrimSpace(apiKey) == "" {
		return 0, errs.New(40020, 400, "API Key 不能为空")
	}
	if strings.TrimSpace(modelName) == "" {
		return 0, errs.New(40022, 400, "模型名称不能为空")
	}

	var url, body string
	if isAnthropic(provider) {
		url = trimBase(baseURL, anthropicDefaultBase) + "/v1/messages"
		body = probeBody(modelName)
	} else {
		base, err := openAIBase(baseURL, provider)
		if err != nil {
			return 0, err
		}
		url = base + "/chat/completions"
		body = probeBody(modelName)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	applyAuthHeaders(req, provider, apiKey)

	start := time.Now()
	if _, err := send(req, "模型测试"); err != nil {
		return 0, err
	}
	return time.Since(start).Milliseconds(), nil
}

func modelsURL(provider, baseURL string) (string, error) {
	if isAnthropic(provider) {
		return trimBase(baseURL, anthropicDefaultBase) + "/v1/models", nil
	}
	base, err := openAIBase(baseURL, provider)
	if err != nil {
		return "", err
	}
	return base + "/models", nil
}

func applyAuthHeaders(req *http.Request, provider, apiKey string) {
	if isAnthropic(provider) {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", anthropicVersion)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func send(req *http.Request, action string) (map[string]any, error) {
	resp, err := probeClient.Do(req)
	if err != nil {
		return nil, errs.New(40023, 400, action+"失败："+err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, errs.New(40023, 400, fmt.Sprintf("%s失败（HTTP %d）：%s", action, resp.StatusCode, snippet(raw)))
	}
	var root map[string]any
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return map[string]any{}, nil
	}
	return root, nil
}

func isAnthropic(provider string) bool { return provider == "anthropic" }

func openAIBase(baseURL, provider string) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		if provider == "openai-compatible" {
			return "", errs.New(40024, 400, "OpenAI 兼容模型需填写 Base URL")
		}
		base = openAIDefaultBase
	}
	base = strings.TrimSuffix(base, "/")
	if !strings.HasSuffix(base, "/v1") {
		base = base + "/v1"
	}
	return base, nil
}

func trimBase(baseURL, defaultBase string) string {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = defaultBase
	}
	return strings.TrimSuffix(base, "/")
}

func probeBody(modelName string) string {
	payload := map[string]any{
		"model":      modelName,
		"max_tokens": 1,
		"messages":   []map[string]any{{"role": "user", "content": "ping"}},
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

func snippet(body []byte) string {
	if len(body) == 0 {
		return "无响应内容"
	}
	if len(body) > maxErrorChars {
		return string(body[:maxErrorChars])
	}
	return string(body)
}
