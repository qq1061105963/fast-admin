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

// Qdrant 是 Qdrant 的 REST 客户端，对应 Java 侧的 AiRagQdrantService。
type Qdrant struct {
	set *settings.Settings
}

func NewQdrant(set *settings.Settings) *Qdrant { return &Qdrant{set: set} }

// SearchHit 是一次召回命中。
type SearchHit struct {
	PointID string
	Score   float64
}

// VectorStoreStatus 对应 AiRagVectorStoreStatusDto。
type VectorStoreStatus struct {
	Enabled                 bool     `json:"enabled"`
	Connected               bool     `json:"connected"`
	URL                     string   `json:"url"`
	Version                 string   `json:"version"`
	Status                  string   `json:"status"`
	DefaultCollection       string   `json:"defaultCollection"`
	DefaultCollectionExists bool     `json:"defaultCollectionExists"`
	Collections             []string `json:"collections"`
	LatencyMs               *int64   `json:"latencyMs"`
	Message                 string   `json:"message"`
}

func (q *Qdrant) Status(ctx context.Context) *VectorStoreStatus {
	status := &VectorStoreStatus{
		Enabled:           q.set.RagEnabled(ctx),
		URL:               q.set.RagQdrantURL(ctx),
		DefaultCollection: q.set.RagCollectionName(ctx),
		Collections:       []string{},
	}
	if !status.Enabled {
		status.Message = "AI 知识库未启用"
		return status
	}
	start := time.Now()
	root, err := q.send(ctx, http.MethodGet, "/", nil)
	if err != nil {
		l := time.Since(start).Milliseconds()
		status.LatencyMs = &l
		status.Message = err.Error()
		return status
	}
	collections, err := q.Collections(ctx)
	if err != nil {
		l := time.Since(start).Milliseconds()
		status.LatencyMs = &l
		status.Message = err.Error()
		return status
	}
	status.Connected = true
	if v, ok := root["version"].(string); ok {
		status.Version = v
	}
	if s, ok := root["status"].(string); ok {
		status.Status = s
	} else {
		status.Status = "ok"
	}
	status.Collections = collections
	status.DefaultCollectionExists = contains(collections, status.DefaultCollection)
	l := time.Since(start).Milliseconds()
	status.LatencyMs = &l
	return status
}

func (q *Qdrant) Collections(ctx context.Context) ([]string, error) {
	root, err := q.send(ctx, http.MethodGet, "/collections", nil)
	if err != nil {
		return nil, err
	}
	result, _ := root["result"].(map[string]any)
	items, _ := result["collections"].([]any)
	if result == nil || items == nil {
		return nil, errs.New(40090, 400, "Qdrant 集合列表返回格式不支持")
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			if name, ok := m["name"].(string); ok && name != "" {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

func (q *Qdrant) EnsureCollection(ctx context.Context, vectorSize int) error {
	collections, err := q.Collections(ctx)
	if err != nil {
		return err
	}
	name := q.set.RagCollectionName(ctx)
	if contains(collections, name) {
		return nil
	}
	body := map[string]any{"vectors": map[string]any{"size": vectorSize, "distance": "Cosine"}}
	_, err = q.send(ctx, http.MethodPut, "/collections/"+name, body)
	return err
}

func (q *Qdrant) Upsert(ctx context.Context, pointID string, vector []float64, payload map[string]any) error {
	name := q.set.RagCollectionName(ctx)
	body := map[string]any{"points": []map[string]any{{"id": pointID, "vector": vector, "payload": payload}}}
	_, err := q.send(ctx, http.MethodPut, "/collections/"+name+"/points?wait=true", body)
	return err
}

func (q *Qdrant) DeletePoints(ctx context.Context, pointIDs []string) error {
	if len(pointIDs) == 0 {
		return nil
	}
	name := q.set.RagCollectionName(ctx)
	collections, err := q.Collections(ctx)
	if err != nil || !contains(collections, name) {
		return err
	}
	body := map[string]any{"points": pointIDs}
	_, err = q.send(ctx, http.MethodPost, "/collections/"+name+"/points/delete?wait=true", body)
	return err
}

func (q *Qdrant) Search(ctx context.Context, vector []float64, knowledgeBaseID string, topK int) ([]SearchHit, error) {
	name := q.set.RagCollectionName(ctx)
	body := map[string]any{
		"vector": vector,
		"limit":  topK,
		"filter": map[string]any{
			"must": []map[string]any{{"key": "knowledgeBaseId", "match": map[string]any{"value": knowledgeBaseID}}},
		},
		"with_payload": true,
	}
	root, err := q.send(ctx, http.MethodPost, "/collections/"+name+"/points/search", body)
	if err != nil {
		return nil, err
	}
	result, ok := root["result"].([]any)
	if !ok {
		return nil, errs.New(40091, 400, "Qdrant 召回结果格式不支持")
	}
	hits := make([]SearchHit, 0, len(result))
	for _, it := range result {
		if m, ok := it.(map[string]any); ok {
			hits = append(hits, SearchHit{PointID: fmt.Sprintf("%v", m["id"]), Score: toFloat(m["score"])})
		}
	}
	return hits, nil
}

func (q *Qdrant) send(ctx context.Context, method, path string, body any) (map[string]any, error) {
	base, err := q.baseURL(ctx)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(q.set.RagQdrantTimeoutMs(ctx)) * time.Millisecond
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, errs.New(40092, 400, "Qdrant 连接失败："+err.Error())
	}
	if apiKey := q.set.RagQdrantAPIKey(ctx); strings.TrimSpace(apiKey) != "" {
		req.Header.Set("api-key", apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errs.New(40092, 400, "Qdrant 连接失败："+err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, errs.New(40093, 400, fmt.Sprintf("Qdrant 请求失败（HTTP %d）：%s", resp.StatusCode, string(raw)))
	}
	var root map[string]any
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, errs.New(40092, 400, "Qdrant 连接失败：响应解析失败")
	}
	return root, nil
}

func (q *Qdrant) baseURL(ctx context.Context) (string, error) {
	url := strings.TrimSpace(q.set.RagQdrantURL(ctx))
	if url == "" {
		return "", errs.New(40094, 400, "Qdrant URL 不能为空")
	}
	return strings.TrimRight(url, "/"), nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}
