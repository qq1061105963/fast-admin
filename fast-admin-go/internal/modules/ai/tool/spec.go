package tool

import (
	"context"
	"encoding/json"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

// Spec 是暴露给模型的一个工具定义 + 调用入口，agent 把它转成 Anthropic/OpenAI 的 tool
// schema，并在调用时包裹 SSE 事件与审计日志。builtin/configured 工具的权限已在 Call
// 闭包里按构建时传入的 permissionCodes 校验；mcp 工具由 mcp 包构建。
type Spec struct {
	Name        string
	Description string
	InputSchema map[string]any
	Source      string // builtin / mcp
	// RequiresConfirmation 为 true 时（execute_sql），agent 需先向前端发 tool_pending
	// 并等待用户确认，确认参数从 ConfirmArgKey 指定的入参里取。
	RequiresConfirmation bool
	ConfirmArgKey        string
	Call                 func(ctx context.Context, args map[string]any) (string, error)
}

// 内置工具的 JSON Schema，对齐 Java 侧 AiToolCallbackService 的常量。
var (
	configuredToolSchema = map[string]any{"type": "object", "additionalProperties": true}

	schemaToolSchema = map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"table": map[string]any{"type": "string", "description": "表名；留空返回当前库所有表名与注释，传入表名返回该表字段定义"},
		},
	}

	readonlySQLSchema = map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"sql"},
		"properties": map[string]any{
			"sql":    map[string]any{"type": "string", "description": "只读 SQL，只允许 select/show/desc/describe/explain，使用 :param 命名参数"},
			"params": map[string]any{"type": "object", "description": "SQL 命名参数，例如 {\"userId\":\"1\"}"},
		},
	}

	executeSQLSchema = map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"sql"},
		"properties": map[string]any{
			"sql":    map[string]any{"type": "string", "description": "任意 SQL 语句，支持 select/insert/update/delete/ddl，使用 :param 命名参数"},
			"params": map[string]any{"type": "object", "description": "SQL 命名参数，例如 {\"userId\":\"1\"}"},
		},
	}
)

const (
	readonlySQLDesc = "执行一条只读 SQL 并返回 JSON 结果。仅用于查询 Fast Admin 数据库事实。\n只允许单条 select/show/desc/describe/explain 语句；需要参数时使用 :param 命名参数并放入 params 对象。"
	schemaToolDesc  = "查询当前数据库的库表结构（仅读取 information_schema 元数据，不涉及业务数据）。\ntable 留空时返回所有表名与注释；传入表名时返回该表的字段名、类型、是否可空、主键与注释。\n在编写 SQL 前可先用此工具确认真实表名与字段，避免猜测。"
	executeSQLDesc  = "执行任意 SQL 并返回结果，支持 select/insert/update/delete/ddl。\n执行前必须先向用户展示 SQL 内容并等待用户确认，用户同意后方可执行。\n查询语句使用 :param 命名参数并放入 params 对象。"
)

// BuildSpecs 按对话模式返回内置 + 配置工具的 Spec 列表，权限已在闭包里校验。
//
// mode: auto=挂载全部已启用工具（含 execute_sql）；manual=仅挂载 selected 里选中的；off=不挂载。
func (s *Service) BuildSpecs(ctx context.Context, exec *Executor, permissionCodes []string, mode string, selected []string) []Spec {
	switch mode {
	case "off":
		return nil
	case "manual":
		includeExecuteSQL := containsCode(selected, ExecuteSQLToolCode)
		return s.listSpecs(ctx, exec, permissionCodes, selected, includeExecuteSQL, true)
	default: // auto
		return s.listSpecs(ctx, exec, permissionCodes, nil, true, false)
	}
}

func (s *Service) listSpecs(ctx context.Context, exec *Executor, permissionCodes, selected []string, includeExecuteSQL, manualMode bool) []Spec {
	var specs []Spec

	// 配置工具
	configured, _ := s.repo.ListEnabled(ctx)
	for i := range configured {
		t := configured[i]
		if !hasPermission(permissionCodes, t.PermissionCode) {
			continue
		}
		if manualMode && !containsCode(selected, t.ToolCode) {
			continue
		}
		code := t.ToolCode
		desc := t.Description
		specs = append(specs, Spec{
			Name: code, Description: desc, InputSchema: configuredToolSchema, Source: "builtin",
			Call: func(ctx context.Context, args map[string]any) (string, error) {
				return exec.Execute(ctx, code, args, permissionCodes)
			},
		})
	}

	// 只读 SQL
	if s.set.ReadonlySQLEnabled(ctx) && hasPermission(permissionCodes, s.set.ReadonlySQLPermissionCode(ctx)) &&
		(!manualMode || containsCode(selected, ReadonlySQLToolCode)) {
		specs = append(specs, Spec{
			Name: ReadonlySQLToolCode, Description: readonlySQLDesc, InputSchema: readonlySQLSchema, Source: "builtin",
			Call: func(ctx context.Context, args map[string]any) (string, error) {
				if err := requirePermission(permissionCodes, s.set.ReadonlySQLPermissionCode(ctx), "无权调用 AI 只读 SQL 工具"); err != nil {
					return "", err
				}
				sql, err := requiredString(args, "sql")
				if err != nil {
					return "", err
				}
				params, err := paramsOf(args)
				if err != nil {
					return "", err
				}
				return exec.ExecuteReadOnlySQL(ctx, sql, params, s.set.ReadonlySQLMaxRows(ctx))
			},
		})
	}

	// 表结构
	if s.set.SchemaToolEnabled(ctx) && hasPermission(permissionCodes, s.set.SchemaToolPermissionCode(ctx)) &&
		(!manualMode || containsCode(selected, SchemaToolCode)) {
		specs = append(specs, Spec{
			Name: SchemaToolCode, Description: schemaToolDesc, InputSchema: schemaToolSchema, Source: "builtin",
			Call: func(ctx context.Context, args map[string]any) (string, error) {
				if err := requirePermission(permissionCodes, s.set.SchemaToolPermissionCode(ctx), "无权调用 AI 查询表结构工具"); err != nil {
					return "", err
				}
				table := ""
				if v, ok := args["table"]; ok && v != nil {
					table = toStr(v)
				}
				return exec.DescribeSchema(ctx, table)
			},
		})
	}

	// 执行 SQL（需二次确认）
	if s.set.ExecuteSQLEnabled(ctx) && includeExecuteSQL && hasPermission(permissionCodes, s.set.ExecuteSQLPermissionCode(ctx)) &&
		(!manualMode || containsCode(selected, ExecuteSQLToolCode)) {
		specs = append(specs, Spec{
			Name: ExecuteSQLToolCode, Description: executeSQLDesc, InputSchema: executeSQLSchema, Source: "builtin",
			RequiresConfirmation: true, ConfirmArgKey: "sql",
			Call: func(ctx context.Context, args map[string]any) (string, error) {
				if err := requirePermission(permissionCodes, s.set.ExecuteSQLPermissionCode(ctx), "无权调用 AI 执行 SQL 工具"); err != nil {
					return "", err
				}
				sql, err := requiredString(args, "sql")
				if err != nil {
					return "", err
				}
				params, err := paramsOf(args)
				if err != nil {
					return "", err
				}
				return exec.ExecuteAnySQL(ctx, sql, params, s.set.ExecuteSQLMaxRows(ctx))
			},
		})
	}
	return specs
}

func hasPermission(permissionCodes []string, requiredCode string) bool {
	if requiredCode == "" {
		return true
	}
	return containsCode(permissionCodes, requiredCode)
}

func requirePermission(permissionCodes []string, requiredCode, msg string) error {
	if requiredCode == "" {
		return nil
	}
	if containsCode(permissionCodes, requiredCode) {
		return nil
	}
	return errs.New(40304, 403, msg)
}

func containsCode(list []string, code string) bool {
	for _, c := range list {
		if c == code {
			return true
		}
	}
	return false
}

func requiredString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", errs.New(40072, 400, "缺少参数："+key)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", errs.New(40072, 400, "缺少参数："+key)
	}
	return s, nil
}

func paramsOf(args map[string]any) (map[string]any, error) {
	v, ok := args["params"]
	if !ok || v == nil {
		return map[string]any{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errs.New(40073, 400, "params 必须是 JSON 对象")
	}
	return m, nil
}

func toStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
