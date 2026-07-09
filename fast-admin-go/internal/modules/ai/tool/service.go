package tool

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/settings"
)

var (
	toolCodePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{1,63}$`)
	types           = map[string]bool{"sql": true, "http": true}
	httpMethods     = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
)

var readonlyPrefixes = []string{"select ", "show ", "desc ", "describe ", "explain "}

type Service struct {
	repo *Repository
	set  *settings.Settings
}

func NewService(repo *Repository, set *settings.Settings) *Service {
	return &Service{repo: repo, set: set}
}

// SaveDTO 对应 AiToolConfigSaveDto。
type SaveDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ToolCode       string `json:"toolCode"`
	Type           string `json:"type"`
	Description    string `json:"description"`
	Enabled        *bool  `json:"enabled"`
	PermissionCode string `json:"permissionCode"`
	Method         string `json:"method"`
	URL            string `json:"url"`
	HeadersJSON    string `json:"headersJson"`
	BodyTemplate   string `json:"bodyTemplate"`
	SQLText        string `json:"sqlText"`
	ReadOnly       *bool  `json:"readOnly"`
	TimeoutMs      *int   `json:"timeoutMs"`
	Remark         string `json:"remark"`
}

// Page 先把内置工具状态同步为系统参数，再分页。
func (s *Service) Page(ctx context.Context, q Query) ([]AiToolConfig, int64, error) {
	s.syncBuiltinTools(ctx)
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) Detail(ctx context.Context, id string) (*AiToolConfig, error) {
	return s.getByIDOrErr(ctx, id)
}

func (s *Service) Add(ctx context.Context, dto *SaveDTO) error {
	if err := s.validate(dto, ""); err != nil {
		return err
	}
	exists, err := s.repo.ToolCodeExists(ctx, "", dto.ToolCode)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if exists {
		return errs.New(40040, 400, "工具编码已存在")
	}
	entity := &AiToolConfig{}
	copyToEntity(dto, entity)
	entity.Enabled = dto.Enabled == nil || *dto.Enabled
	entity.SystemBuiltin = false
	if err := s.repo.Create(ctx, entity); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, dto *SaveDTO) error {
	entity, err := s.getByIDOrErr(ctx, dto.ID)
	if err != nil {
		return err
	}
	if entity.SystemBuiltin {
		return errs.New(40041, 400, "系统内置工具不允许编辑")
	}
	if err := s.validate(dto, entity.ID); err != nil {
		return err
	}
	exists, err := s.repo.ToolCodeExists(ctx, entity.ID, dto.ToolCode)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if exists {
		return errs.New(40040, 400, "工具编码已存在")
	}
	copyToEntity(dto, entity)
	if err := s.repo.Update(ctx, entity); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) ChangeEnabled(ctx context.Context, id string, enabled bool) error {
	entity, err := s.getByIDOrErr(ctx, id)
	if err != nil {
		return err
	}
	if entity.SystemBuiltin {
		return errs.New(40042, 400, "内置工具的启用状态由系统参数控制，请在系统参数中调整")
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
	if entity.SystemBuiltin {
		return errs.New(40043, 400, "系统内置工具不允许删除")
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

// ListEnabled 供 agent 构建工具列表。
func (s *Service) ListEnabled(ctx context.Context) ([]AiToolConfig, error) {
	return s.repo.ListEnabled(ctx)
}

func (s *Service) getByIDOrErr(ctx context.Context, id string) (*AiToolConfig, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.New(40404, 404, "AI 工具不存在")
	}
	return t, nil
}

func (s *Service) syncBuiltinTools(ctx context.Context) {
	_ = s.repo.SyncBuiltin(ctx, ReadonlySQLToolCode, s.set.ReadonlySQLEnabled(ctx), s.set.ReadonlySQLPermissionCode(ctx))
	_ = s.repo.SyncBuiltin(ctx, ExecuteSQLToolCode, s.set.ExecuteSQLEnabled(ctx), s.set.ExecuteSQLPermissionCode(ctx))
	_ = s.repo.SyncBuiltin(ctx, SchemaToolCode, s.set.SchemaToolEnabled(ctx), s.set.SchemaToolPermissionCode(ctx))
}

func (s *Service) validate(dto *SaveDTO, id string) error {
	if dto == nil {
		return errs.New(40044, 400, "AI 工具配置不能为空")
	}
	if strings.TrimSpace(dto.Name) == "" {
		return errs.New(40045, 400, "工具名称不能为空")
	}
	if !toolCodePattern.MatchString(dto.ToolCode) {
		return errs.New(40046, 400, "工具编码只能使用英文字母、数字和下划线，并且以字母开头")
	}
	if !types[dto.Type] {
		return errs.New(40047, 400, "工具类型不支持")
	}
	if strings.TrimSpace(dto.Description) == "" {
		return errs.New(40048, 400, "工具说明不能为空")
	}
	if dto.Type == "sql" {
		return validateSQL(dto)
	}
	return s.validateHTTP(dto)
}

func validateSQL(dto *SaveDTO) error {
	if strings.TrimSpace(dto.SQLText) == "" {
		return errs.New(40049, 400, "SQL 工具必须填写 SQL 模板")
	}
	if hasMultipleStatements(dto.SQLText) {
		return errs.New(40050, 400, "SQL 工具只允许配置单条语句")
	}
	readOnly := dto.ReadOnly == nil || *dto.ReadOnly
	if readOnly && !isReadOnlySQL(dto.SQLText) {
		return errs.New(40051, 400, "只读 SQL 工具仅允许 select/show/desc/describe/explain")
	}
	return nil
}

func (s *Service) validateHTTP(dto *SaveDTO) error {
	method := strings.ToUpper(strings.TrimSpace(dto.Method))
	if !httpMethods[method] {
		return errs.New(40052, 400, "HTTP 方法不支持")
	}
	if !strings.HasPrefix(dto.URL, "http://") && !strings.HasPrefix(dto.URL, "https://") {
		return errs.New(40053, 400, "HTTP 地址必须以 http:// 或 https:// 开头")
	}
	if strings.TrimSpace(dto.HeadersJSON) != "" {
		var node map[string]any
		if err := json.Unmarshal([]byte(dto.HeadersJSON), &node); err != nil {
			return errs.New(40054, 400, "请求头 JSON 必须是对象")
		}
	}
	return nil
}

func copyToEntity(dto *SaveDTO, entity *AiToolConfig) {
	entity.Name = dto.Name
	entity.ToolCode = dto.ToolCode
	entity.Type = dto.Type
	entity.Description = dto.Description
	if dto.Enabled != nil {
		entity.Enabled = *dto.Enabled
	}
	entity.PermissionCode = dto.PermissionCode
	if strings.TrimSpace(dto.Method) != "" {
		entity.Method = strings.ToUpper(dto.Method)
	} else {
		entity.Method = ""
	}
	entity.URL = dto.URL
	entity.HeadersJSON = dto.HeadersJSON
	entity.BodyTemplate = dto.BodyTemplate
	entity.SQLText = dto.SQLText
	entity.ReadOnly = dto.ReadOnly == nil || *dto.ReadOnly
	if dto.TimeoutMs == nil {
		entity.TimeoutMs = 10000
	} else {
		entity.TimeoutMs = *dto.TimeoutMs
	}
	entity.Remark = dto.Remark
}

func hasMultipleStatements(sqlText string) bool {
	normalized := strings.TrimSpace(sqlText)
	normalized = strings.TrimSuffix(normalized, ";")
	return strings.Contains(normalized, ";")
}

func isReadOnlySQL(sqlText string) bool {
	normalized := strings.ToLower(strings.TrimLeft(sqlText, " \t\r\n"))
	for _, p := range readonlyPrefixes {
		if strings.HasPrefix(normalized, p) {
			return true
		}
	}
	return false
}
