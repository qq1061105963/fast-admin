package model

import (
	"context"
	"strings"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

const mask = "******"

var providers = map[string]bool{"anthropic": true, "openai": true, "openai-compatible": true}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// DTO 是返回给前端的模型配置，api_key 脱敏。
type DTO struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	BaseURL       string   `json:"baseUrl"`
	APIKey        string   `json:"apiKey"`
	Enabled       bool     `json:"enabled"`
	Active        bool     `json:"active"`
	Temperature   *float64 `json:"temperature"`
	MaxTokens     *int     `json:"maxTokens"`
	Remark        string   `json:"remark"`
	LastLatencyMs *int64   `json:"lastLatencyMs"`
	LastTestOk    *bool    `json:"lastTestOk"`
	LastTestedAt  string   `json:"lastTestedAt"`
	CreatedAt     string   `json:"createdAt"`
}

// SaveDTO 对应 AiModelConfigSaveDto。
type SaveDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
	Model       string   `json:"model"`
	BaseURL     string   `json:"baseUrl"`
	APIKey      string   `json:"apiKey"`
	Enabled     *bool    `json:"enabled"`
	Active      *bool    `json:"active"`
	Temperature *float64 `json:"temperature"`
	MaxTokens   *int     `json:"maxTokens"`
	Remark      string   `json:"remark"`
}

// TestResult 对应 AiModelTestResultDto。
type TestResult struct {
	Success   bool   `json:"success"`
	LatencyMs int64  `json:"latencyMs"`
	Message   string `json:"message"`
}

func (s *Service) Page(ctx context.Context, q Query) ([]DTO, int64, error) {
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	dtos := make([]DTO, 0, len(list))
	for i := range list {
		dtos = append(dtos, toDTO(&list[i]))
	}
	return dtos, total, nil
}

func (s *Service) Detail(ctx context.Context, id string) (*DTO, error) {
	m, err := s.getByIDOrErr(ctx, id)
	if err != nil {
		return nil, err
	}
	d := toDTO(m)
	return &d, nil
}

// GetActiveEnabled 供 agent 读取当前模型；无激活模型返回 (nil, nil)。
func (s *Service) GetActiveEnabled(ctx context.Context) (*AiModelConfig, error) {
	m, err := s.repo.GetActiveEnabled(ctx)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return m, nil
}

func (s *Service) Add(ctx context.Context, dto *SaveDTO) error {
	if err := validate(dto, ""); err != nil {
		return err
	}
	exists, err := s.repo.NameExists(ctx, "", dto.Name)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if exists {
		return errs.New(40025, 400, "模型配置名称已存在")
	}
	entity := &AiModelConfig{}
	copyToEntity(dto, entity, "")
	entity.Enabled = dto.Enabled == nil || *dto.Enabled
	entity.Active = dto.Active != nil && *dto.Active
	if err := s.repo.Create(ctx, entity); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if entity.Active {
		return s.Activate(ctx, entity.ID)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, dto *SaveDTO) error {
	entity, err := s.getByIDOrErr(ctx, dto.ID)
	if err != nil {
		return err
	}
	if err := validate(dto, entity.ID); err != nil {
		return err
	}
	exists, err := s.repo.NameExists(ctx, entity.ID, dto.Name)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if exists {
		return errs.New(40025, 400, "模型配置名称已存在")
	}
	copyToEntity(dto, entity, entity.ID)
	if err := s.repo.Update(ctx, entity); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if dto.Active != nil && *dto.Active {
		return s.Activate(ctx, entity.ID)
	}
	return nil
}

// Activate 把目标设为唯一激活模型，禁用的配置不允许激活。
func (s *Service) Activate(ctx context.Context, id string) error {
	target, err := s.getByIDOrErr(ctx, id)
	if err != nil {
		return err
	}
	if !target.Enabled {
		return errs.New(40026, 400, "禁用的模型配置不能设为当前模型")
	}
	if err := s.repo.ClearActive(ctx); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if err := s.repo.SetActive(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) ChangeEnabled(ctx context.Context, id string, enabled bool) error {
	entity, err := s.getByIDOrErr(ctx, id)
	if err != nil {
		return err
	}
	entity.Enabled = enabled
	if err := s.repo.Update(ctx, entity); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) Del(ctx context.Context, id string) error {
	entity, err := s.getByIDOrErr(ctx, id)
	if err != nil {
		return err
	}
	if entity.Active {
		return errs.New(40027, 400, "当前激活模型不能删除")
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) FetchModels(ctx context.Context, dto *SaveDTO) ([]string, error) {
	if err := validateProvider(dto); err != nil {
		return nil, err
	}
	apiKey, err := s.resolveAPIKey(ctx, dto)
	if err != nil {
		return nil, err
	}
	return FetchModels(ctx, dto.Provider, dto.BaseURL, apiKey)
}

// Test 测试连通性；已存在的配置会记录本次结果。
func (s *Service) Test(ctx context.Context, dto *SaveDTO) (*TestResult, error) {
	if err := validateProvider(dto); err != nil {
		return nil, err
	}
	if strings.TrimSpace(dto.Model) == "" {
		return nil, errs.New(40022, 400, "模型名称不能为空")
	}
	apiKey, err := s.resolveAPIKey(ctx, dto)
	if err != nil {
		return nil, err
	}
	latency, err := Test(ctx, dto.Provider, dto.BaseURL, apiKey, dto.Model)
	if err != nil {
		if dto.ID != "" {
			_ = s.repo.RecordTestResult(ctx, dto.ID, nil, false)
		}
		return nil, err
	}
	if dto.ID != "" {
		l := latency
		_ = s.repo.RecordTestResult(ctx, dto.ID, &l, true)
	}
	return &TestResult{Success: true, LatencyMs: latency}, nil
}

func (s *Service) getByIDOrErr(ctx context.Context, id string) (*AiModelConfig, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.New(40404, 404, "模型配置不存在")
	}
	return m, nil
}

// resolveAPIKey 优先用页面填写的密钥，掩码/空时回退已存储密钥。
func (s *Service) resolveAPIKey(ctx context.Context, dto *SaveDTO) (string, error) {
	if strings.TrimSpace(dto.APIKey) != "" && dto.APIKey != mask {
		return dto.APIKey, nil
	}
	if dto.ID != "" {
		stored, err := s.repo.GetByID(ctx, dto.ID)
		if err == nil && strings.TrimSpace(stored.APIKey) != "" {
			return stored.APIKey, nil
		}
	}
	return "", errs.New(40020, 400, "API Key 不能为空")
}

func validateProvider(dto *SaveDTO) error {
	if dto == nil {
		return errs.New(40028, 400, "模型配置不能为空")
	}
	if !providers[dto.Provider] {
		return errs.New(40029, 400, "模型提供方不支持")
	}
	return nil
}

func validate(dto *SaveDTO, id string) error {
	if dto == nil {
		return errs.New(40028, 400, "模型配置不能为空")
	}
	if strings.TrimSpace(dto.Name) == "" {
		return errs.New(40030, 400, "模型配置名称不能为空")
	}
	if !providers[dto.Provider] {
		return errs.New(40029, 400, "模型提供方不支持")
	}
	if strings.TrimSpace(dto.Model) == "" {
		return errs.New(40022, 400, "模型名称不能为空")
	}
	if id == "" && strings.TrimSpace(dto.APIKey) == "" {
		return errs.New(40020, 400, "API Key 不能为空")
	}
	return nil
}

func copyToEntity(dto *SaveDTO, entity *AiModelConfig, id string) {
	entity.Name = dto.Name
	entity.Provider = dto.Provider
	entity.Model = dto.Model
	entity.BaseURL = dto.BaseURL
	if strings.TrimSpace(dto.APIKey) != "" && dto.APIKey != mask {
		entity.APIKey = dto.APIKey
	}
	if dto.Enabled != nil {
		entity.Enabled = *dto.Enabled
	}
	entity.Active = dto.Active != nil && *dto.Active
	entity.Temperature = dto.Temperature
	entity.MaxTokens = dto.MaxTokens
	entity.Remark = dto.Remark
	_ = id
}

func toDTO(m *AiModelConfig) DTO {
	d := DTO{
		ID:            m.ID,
		Name:          m.Name,
		Provider:      m.Provider,
		Model:         m.Model,
		BaseURL:       m.BaseURL,
		Enabled:       m.Enabled,
		Active:        m.Active,
		Temperature:   m.Temperature,
		MaxTokens:     m.MaxTokens,
		Remark:        m.Remark,
		LastLatencyMs: m.LastLatencyMs,
		LastTestOk:    m.LastTestOk,
	}
	if strings.TrimSpace(m.APIKey) != "" {
		d.APIKey = mask
	}
	if m.LastTestedAt != nil {
		d.LastTestedAt = m.LastTestedAt.Format("2006-01-02T15:04:05")
	}
	if !m.CreatedAt.IsZero() {
		d.CreatedAt = m.CreatedAt.Format("2006-01-02T15:04:05")
	}
	return d
}
