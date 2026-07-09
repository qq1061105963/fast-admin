package rag

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
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/settings"
)

// Embedding 是 OpenAI 兼容的 embedding 客户端，对应 Java 侧的 AiRagEmbeddingService。
type Embedding struct {
	set *settings.Settings
}

func NewEmbedding(set *settings.Settings) *Embedding { return &Embedding{set: set} }

// Embed 请求 /embeddings，返回单条向量。
func (e *Embedding) Embed(ctx context.Context, input string) ([]float64, error) {
	if strings.TrimSpace(input) == "" {
		return nil, errs.New(40080, 400, "Embedding 文本不能为空")
	}
	baseURL := e.set.RagEmbeddingBaseURL(ctx)
	apiKey := e.set.RagEmbeddingAPIKey(ctx)
	model := e.set.RagEmbeddingModel(ctx)
	timeoutMs := e.set.RagEmbeddingTimeoutMs(ctx)
	if strings.TrimSpace(baseURL) == "" {
		return nil, errs.New(40081, 400, "Embedding Base URL 不能为空，请在 AI 运维的 AI 配置页面配置。RAG 需要独立的 embedding 模型，不能直接复用当前聊天模型")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errs.New(40085, 400, "Embedding API Key 不能为空，请在 AI 运维的 AI 配置页面配置")
	}
	base := normalizeEmbeddingBaseURL(baseURL)

	payload, _ := json.Marshal(map[string]any{"model": model, "input": input})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, errs.New(40082, 400, "Embedding 请求失败："+err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errs.New(40082, 400, "Embedding 请求失败："+err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, errs.New(40082, 400, fmt.Sprintf(
			"Embedding 请求失败（HTTP %d）：请检查 Base URL 是否支持 /v1/embeddings，模型是否为 embedding 模型。%s",
			resp.StatusCode, snippetStr(raw)))
	}
	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, errs.New(40083, 400, "Embedding 返回格式不支持")
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, errs.New(40084, 400, "Embedding 向量为空")
	}
	return parsed.Data[0].Embedding, nil
}

// normalizeEmbeddingBaseURL 去掉结尾斜杠并保证以 /v1 结尾，对齐 Java 侧的 resolveConfig 逻辑。
func normalizeEmbeddingBaseURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if !strings.HasSuffix(base, "/v1") {
		base = base + "/v1"
	}
	return base
}

func snippetStr(raw []byte) string {
	s := string(raw)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}
