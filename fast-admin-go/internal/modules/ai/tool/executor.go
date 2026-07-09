package tool

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

// Executor 执行 AI 工具（配置的 sql/http 工具，以及内置 SQL / 表结构工具），
// 对应 Java 侧的 AiToolExecutionService。所有 SQL 走 GORM 原生执行。
type Executor struct {
	repo *Repository
	db   *gorm.DB
}

func NewExecutor(repo *Repository, db *gorm.DB) *Executor {
	return &Executor{repo: repo, db: db}
}

const (
	maxQueryRows    = 100
	maxHTTPBodyChar = 8000
)

var (
	templateParam        = regexp.MustCompile(`\{\{\s*([A-Za-z][A-Za-z0-9_]*)\s*}}`)
	namedParam           = regexp.MustCompile(`:([A-Za-z][A-Za-z0-9_]*)`)
	schemaIdentifier     = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	sensitiveColumnInSQL = regexp.MustCompile(`(?i)\b(api_key|apikey|password|passwd|secret|token|private_key)\b`)
)

var sensitiveKeywords = []string{"api_key", "apikey", "password", "passwd", "secret", "token", "private_key"}

// Execute 执行配置的工具（sql/http）。
func (e *Executor) Execute(ctx context.Context, toolCode string, args map[string]any, permissionCodes []string) (string, error) {
	t, err := e.repo.GetEnabledByToolCode(ctx, toolCode)
	if err != nil {
		return "", errs.ErrInternal.Wrap(err)
	}
	if t == nil {
		return "", errs.New(40060, 400, "AI 工具未启用或不存在："+toolCode)
	}
	if err := checkPermission(t, permissionCodes); err != nil {
		return "", err
	}
	if args == nil {
		args = map[string]any{}
	}
	if t.Type == "sql" {
		return e.executeConfiguredSQL(ctx, t, args)
	}
	return e.executeHTTP(ctx, t, args)
}

// ExecuteAnySQL 执行任意 SQL（execute_sql 内置工具）。查询语句限制返回行数，写语句返回影响行数。
func (e *Executor) ExecuteAnySQL(ctx context.Context, sqlText string, params map[string]any, maxRows int) (string, error) {
	if strings.TrimSpace(sqlText) == "" {
		return "", errs.New(40061, 400, "SQL 不能为空")
	}
	if err := rejectSensitiveColumns(sqlText); err != nil {
		return "", err
	}
	normalized := strings.ToLower(strings.TrimSpace(sqlText))
	normalized = strings.TrimSpace(strings.TrimSuffix(normalized, ";"))
	isQuery := hasReadonlyPrefix(normalized)
	if isQuery {
		limit := clampInt(maxRows, 1, maxQueryRows)
		return e.queryFormatted(ctx, sqlText, params, limit, true)
	}
	affected, err := e.exec(ctx, sqlText, params)
	if err != nil {
		return "", errs.New(40062, 400, "SQL 执行失败："+err.Error())
	}
	return fmt.Sprintf("SQL 执行完成，影响行数：%d", affected), nil
}

// ExecuteReadOnlySQL 执行只读 SQL（execute_readonly_sql 内置工具）。
func (e *Executor) ExecuteReadOnlySQL(ctx context.Context, sqlText string, params map[string]any, maxRows int) (string, error) {
	if err := rejectSensitiveColumns(sqlText); err != nil {
		return "", err
	}
	if err := validateReadOnlySQL(sqlText); err != nil {
		return "", err
	}
	limit := clampInt(maxRows, 1, maxQueryRows)
	return e.queryFormatted(ctx, sqlText, params, limit, true)
}

// DescribeSchema 读取当前库表结构，只查 information_schema。table 为空返回所有表。
func (e *Executor) DescribeSchema(ctx context.Context, table string) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		rows, _, _, err := e.rawQuery(ctx,
			"SELECT table_name AS tableName, table_comment AS tableComment "+
				"FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY table_name",
			nil, 10000)
		if err != nil {
			return "", errs.New(40063, 400, "查询表结构失败："+err.Error())
		}
		return toJSON(map[string]any{"tableCount": len(rows), "tables": rows}), nil
	}
	if !schemaIdentifier.MatchString(table) {
		return "", errs.New(40064, 400, "表名仅允许字母、数字、下划线："+table)
	}
	rows, _, _, err := e.rawQuery(ctx,
		"SELECT column_name AS columnName, column_type AS columnType, is_nullable AS nullable, "+
			"column_key AS columnKey, column_default AS columnDefault, column_comment AS columnComment "+
			"FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = :table "+
			"ORDER BY ordinal_position",
		map[string]any{"table": table}, 10000)
	if err != nil {
		return "", errs.New(40063, 400, "查询表结构失败："+err.Error())
	}
	if len(rows) == 0 {
		return "", errs.New(40065, 400, "表不存在或没有字段："+table)
	}
	return toJSON(map[string]any{"table": table, "columnCount": len(rows), "columns": rows}), nil
}

func (e *Executor) executeConfiguredSQL(ctx context.Context, t *AiToolConfig, args map[string]any) (string, error) {
	if !t.ReadOnly {
		affected, err := e.exec(ctx, t.SQLText, args)
		if err != nil {
			return "", errs.New(40062, 400, "SQL 执行失败："+err.Error())
		}
		return fmt.Sprintf("SQL 执行完成，影响行数：%d", affected), nil
	}
	return e.queryFormatted(ctx, t.SQLText, args, maxQueryRows, false)
}

// queryFormatted 查询并格式化为 JSON 字符串，includeReturned 控制是否输出 returned 计数。
func (e *Executor) queryFormatted(ctx context.Context, sqlText string, params map[string]any, limit int, includeReturned bool) (string, error) {
	rows, total, truncated, err := e.rawQuery(ctx, sqlText, params, limit)
	if err != nil {
		return "", errs.New(40066, 400, "SQL 查询失败："+err.Error())
	}
	result := map[string]any{"total": total, "truncated": truncated, "rows": maskRows(rows)}
	if includeReturned {
		result["returned"] = len(rows)
	}
	return toJSON(result), nil
}

// rawQuery 执行查询，最多读取 limit+1 行以判断是否截断，返回前 limit 行。
func (e *Executor) rawQuery(ctx context.Context, sqlText string, params map[string]any, limit int) ([]map[string]any, int, bool, error) {
	converted, args := bindNamed(sqlText, params)
	sqlRows, err := e.db.WithContext(ctx).Raw(converted, args...).Rows()
	if err != nil {
		return nil, 0, false, err
	}
	defer sqlRows.Close()

	cols, err := sqlRows.Columns()
	if err != nil {
		return nil, 0, false, err
	}
	var all []map[string]any
	for sqlRows.Next() {
		if len(all) >= limit+1 {
			break
		}
		holders := make([]any, len(cols))
		for i := range holders {
			holders[i] = new(any)
		}
		if err := sqlRows.Scan(holders...); err != nil {
			return nil, 0, false, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalizeValue(*(holders[i].(*any)))
		}
		all = append(all, row)
	}
	totalRead := len(all)
	truncated := totalRead > limit
	if truncated {
		all = all[:limit]
	}
	return all, totalRead, truncated, nil
}

func (e *Executor) exec(ctx context.Context, sqlText string, params map[string]any) (int64, error) {
	converted, args := bindNamed(sqlText, params)
	tx := e.db.WithContext(ctx).Exec(converted, args...)
	return tx.RowsAffected, tx.Error
}

func (e *Executor) executeHTTP(ctx context.Context, t *AiToolConfig, args map[string]any) (string, error) {
	timeoutMs := 10000
	if t.TimeoutMs > 0 {
		timeoutMs = clampInt(t.TimeoutMs, 1000, 60000)
	}
	client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}

	target := renderTemplate(t.URL, args, true)
	method := strings.ToUpper(strings.TrimSpace(t.Method))
	if method == "" {
		method = "GET"
	}
	var bodyReader io.Reader
	if method != "GET" && method != "DELETE" {
		bodyReader = strings.NewReader(renderTemplate(t.BodyTemplate, args, false))
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return "", errs.New(40067, 400, "HTTP 工具调用失败："+err.Error())
	}
	if err := applyHeaders(req, t.HeadersJSON, args); err != nil {
		return "", errs.New(40067, 400, "HTTP 工具调用失败："+err.Error())
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errs.New(40067, 400, "HTTP 工具调用失败："+err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	body := string(raw)
	if len(body) > maxHTTPBodyChar {
		body = body[:maxHTTPBodyChar]
	}
	return toJSON(map[string]any{"status": resp.StatusCode, "body": body}), nil
}

func applyHeaders(req *http.Request, headersJSON string, args map[string]any) error {
	if strings.TrimSpace(headersJSON) == "" {
		return nil
	}
	var headers map[string]any
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		return err
	}
	for k, v := range headers {
		if v != nil {
			req.Header.Set(k, renderTemplate(fmt.Sprintf("%v", v), args, false))
		}
	}
	return nil
}

func checkPermission(t *AiToolConfig, permissionCodes []string) error {
	if strings.TrimSpace(t.PermissionCode) == "" {
		return nil
	}
	for _, c := range permissionCodes {
		if c == t.PermissionCode {
			return nil
		}
	}
	return errs.New(40303, 403, "无权调用 AI 工具："+t.Name)
}

func validateReadOnlySQL(sqlText string) error {
	if strings.TrimSpace(sqlText) == "" {
		return errs.New(40061, 400, "SQL 不能为空")
	}
	normalized := strings.TrimSpace(sqlText)
	if strings.Contains(normalized, "--") || strings.Contains(normalized, "#") || strings.Contains(normalized, "/*") {
		return errs.New(40068, 400, "只读 SQL 不允许包含注释")
	}
	if hasMultipleStatements(normalized) {
		return errs.New(40069, 400, "只读 SQL 只允许单条语句")
	}
	lower := strings.ToLower(normalized)
	lower = strings.TrimSpace(strings.TrimSuffix(lower, ";"))
	if !hasReadonlyPrefix(lower) {
		return errs.New(40070, 400, "只读 SQL 仅允许 select/show/desc/describe/explain")
	}
	return nil
}

func rejectSensitiveColumns(sqlText string) error {
	if sensitiveColumnInSQL.MatchString(sqlText) {
		return errs.New(40071, 400, "SQL 引用了敏感字段，禁止查询")
	}
	return nil
}

func maskRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		masked := make(map[string]any, len(row))
		for col, val := range row {
			if val == nil {
				masked[col] = nil
				continue
			}
			lower := strings.ToLower(col)
			sensitive := false
			for _, kw := range sensitiveKeywords {
				if strings.Contains(lower, kw) {
					sensitive = true
					break
				}
			}
			if sensitive {
				masked[col] = "******"
			} else {
				masked[col] = val
			}
		}
		out = append(out, masked)
	}
	return out
}

func hasReadonlyPrefix(normalized string) bool {
	for _, p := range readonlyPrefixes {
		if strings.HasPrefix(normalized, p) {
			return true
		}
	}
	return false
}

// bindNamed 把 :name 命名参数转成 ? 位置参数，按出现顺序收集实参，
// 语义对齐 Spring 的 NamedParameterJdbcTemplate（同名可重复出现）。
func bindNamed(sqlText string, params map[string]any) (string, []any) {
	if params == nil {
		params = map[string]any{}
	}
	var args []any
	converted := namedParam.ReplaceAllStringFunc(sqlText, func(m string) string {
		name := m[1:]
		args = append(args, params[name])
		return "?"
	})
	return converted, args
}

func renderTemplate(template string, args map[string]any, urlEncode bool) string {
	if strings.TrimSpace(template) == "" {
		return template
	}
	return templateParam.ReplaceAllStringFunc(template, func(m string) string {
		sub := templateParam.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		val := args[sub[1]]
		text := ""
		if val != nil {
			text = fmt.Sprintf("%v", val)
		}
		if urlEncode {
			text = url.QueryEscape(text)
		}
		return text
	})
}

// normalizeValue 把驱动返回的 []byte（MySQL 文本协议常见）转成字符串，便于 JSON 序列化。
func normalizeValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case sql.RawBytes:
		return string(t)
	default:
		return v
	}
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
