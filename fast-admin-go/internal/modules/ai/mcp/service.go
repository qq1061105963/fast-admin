package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

var transports = map[string]bool{"stdio": true, "sse": true, "streamable-http": true}

const (
	minKeepAliveSeconds     = 5
	maxKeepAliveSeconds     = 3600
	defaultKeepAliveSeconds = 30
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// SaveDTO 对应 AiMcpServerSaveDto。
type SaveDTO struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Transport         string `json:"transport"`
	Command           string `json:"command"`
	URL               string `json:"url"`
	ArgsJSON          string `json:"argsJson"`
	HeadersJSON       string `json:"headersJson"`
	Enabled           *bool  `json:"enabled"`
	KeepAlive         *bool  `json:"keepAlive"`
	KeepAliveInterval *int   `json:"keepAliveInterval"`
	Remark            string `json:"remark"`
}

func (s *Service) Page(ctx context.Context, q Query) ([]AiMcpServer, int64, error) {
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) GetByIDOrErr(ctx context.Context, id string) (*AiMcpServer, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.New(40404, 404, "MCP 服务不存在")
	}
	return m, nil
}

func (s *Service) Add(ctx context.Context, dto *SaveDTO) (*AiMcpServer, error) {
	if err := s.validate(dto); err != nil {
		return nil, err
	}
	exists, err := s.repo.NameExists(ctx, "", dto.Name)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	if exists {
		return nil, errs.New(40130, 400, "MCP 服务名称已存在")
	}
	entity := &AiMcpServer{}
	copyToEntity(dto, entity)
	entity.Enabled = dto.Enabled == nil || *dto.Enabled
	if err := s.repo.Create(ctx, entity); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return entity, nil
}

func (s *Service) Update(ctx context.Context, dto *SaveDTO) error {
	entity, err := s.GetByIDOrErr(ctx, dto.ID)
	if err != nil {
		return err
	}
	if err := s.validate(dto); err != nil {
		return err
	}
	exists, err := s.repo.NameExists(ctx, entity.ID, dto.Name)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if exists {
		return errs.New(40130, 400, "MCP 服务名称已存在")
	}
	copyToEntity(dto, entity)
	if err := s.repo.Update(ctx, entity); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) ChangeEnabled(ctx context.Context, id string, enabled bool) error {
	entity, err := s.GetByIDOrErr(ctx, id)
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
	if _, err := s.GetByIDOrErr(ctx, id); err != nil {
		return err
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) ListEnabled(ctx context.Context) ([]AiMcpServer, error) {
	return s.repo.ListEnabled(ctx)
}

func (s *Service) validate(dto *SaveDTO) error {
	if dto == nil {
		return errs.New(40131, 400, "MCP 服务配置不能为空")
	}
	if strings.TrimSpace(dto.Name) == "" {
		return errs.New(40132, 400, "MCP 服务名称不能为空")
	}
	if !transports[dto.Transport] {
		return errs.New(40133, 400, "MCP 传输类型不支持")
	}
	if dto.Transport == "stdio" && strings.TrimSpace(dto.Command) == "" {
		return errs.New(40134, 400, "stdio 模式命令不能为空")
	}
	if dto.Transport != "stdio" && strings.TrimSpace(dto.URL) == "" {
		return errs.New(40135, 400, "远程 MCP 地址不能为空")
	}
	if err := validateJSON(dto.ArgsJSON, true, "命令参数 JSON 必须是数组"); err != nil {
		return err
	}
	if err := validateJSON(dto.HeadersJSON, false, "请求头 JSON 必须是对象"); err != nil {
		return err
	}
	if dto.Transport == "sse" && dto.KeepAlive != nil && *dto.KeepAlive {
		if dto.KeepAliveInterval == nil || *dto.KeepAliveInterval < minKeepAliveSeconds || *dto.KeepAliveInterval > maxKeepAliveSeconds {
			return errs.New(40136, 400, "保活间隔需在 5~3600 秒之间")
		}
	}
	return nil
}

func validateJSON(jsonStr string, array bool, msg string) error {
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return errs.New(40137, 400, msg)
	}
	if array {
		if _, ok := v.([]any); !ok {
			return errs.New(40137, 400, msg)
		}
	} else {
		if _, ok := v.(map[string]any); !ok {
			return errs.New(40137, 400, msg)
		}
	}
	return nil
}

func copyToEntity(dto *SaveDTO, entity *AiMcpServer) {
	entity.Name = dto.Name
	entity.Transport = dto.Transport
	entity.Command = dto.Command
	entity.URL = dto.URL
	entity.ArgsJSON = dto.ArgsJSON
	entity.HeadersJSON = dto.HeadersJSON
	if dto.Enabled != nil {
		entity.Enabled = *dto.Enabled
	}
	// 保活仅对 sse 生效，其余传输类型强制关闭
	entity.KeepAlive = dto.Transport == "sse" && dto.KeepAlive != nil && *dto.KeepAlive
	if dto.KeepAliveInterval == nil {
		entity.KeepAliveInterval = defaultKeepAliveSeconds
	} else {
		entity.KeepAliveInterval = *dto.KeepAliveInterval
	}
	entity.Remark = dto.Remark
}
