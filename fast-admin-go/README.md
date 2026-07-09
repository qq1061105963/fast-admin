# fast-admin-go

fast-admin 后端的 Go 重写。已完整覆盖登录认证 + `system` 下的全部功能模块
（user/role/menu/permission/online/dept/dict/config/file/job/log），对应 Java 侧的
`fast-framework` + `fast-system`；**AI 助手模块（fast-ai）也已完整复刻**为
`internal/modules/ai/**`（config/model/tool/rag/mcp/agent + settings），单二进制内直连，
不再依赖 Java 服务。工作流（fast-flow）模块仍不在本次范围内（两个 Go 生态都没有成熟的
BPMN 引擎），建议保留 Java 服务、Go 侧通过 REST/gRPC 调用。

AI 侧不再依赖 Spring AI：聊天走手写的流式 + 工具调用循环（`agent/llm.go`），直连
Anthropic Messages / OpenAI Chat Completions；MCP 用官方 Go SDK
（`github.com/modelcontextprotocol/go-sdk`）；RAG 的 embedding/Qdrant 都是裸 HTTP，
文档抽取用 `excelize` + 内置 zip/xml 解析。

直接复用现有的 `scripts/sql/fast_admin_init.sql` 建表，**不做 AutoMigrate**——Go 侧
模型字段和现有表结构一一对应，启动时按现有 schema 连接即可，不会尝试改表。

## 目录结构

```
cmd/server/main.go              程序入口，对应 fast-application
configs/                        多 profile 配置
internal/framework/             对应 fast-framework，通用能力
  config/       viper 配置加载
  logger/       zap 日志封装
  database/     GORM 多数据源初始化
  auth/         token/session 鉴权（替代 Sa-Token，支持多端并发登录）
  audit/        当前操作人 context 透传，供 BaseModel 钩子回填审计字段
  middleware/   recovery / 请求日志 / CORS / 鉴权 / 操作日志采集 / traceId
  errs/         统一业务错误类型
  response/     统一响应结构（Rs/Ps，对齐现有 {code,message,data,timestamp,traceId}）
  oplog/        操作日志 Entry 类型 + 脱敏规则（被中间件和 syslog 模块共用）
  loginlog/     登录日志 Entry 类型（被 authn 和 syslog 模块共用）
  security/     bcrypt 密码哈希
  useragent/    User-Agent 粗略解析（浏览器/系统/设备类型）
  model/        BaseModel（KSUID 主键 + 审计字段 + 软删除）/ LogModel（纯日志表）
  crud/         泛型 BaseRepo[T]，对应 MyBatis-Plus 的 BaseMapper/IService
  datascope/    行级数据权限过滤（GORM Scope），替代原来的 AOP 切面
internal/modules/
  permission/   sys_roles_menus / sys_users_roles 中间表 repository（无独立路由）
  menu/         菜单树构建、用户菜单、权限码
  role/         角色 + 自定义数据范围（sys_role_dept）
  dept/         部门树 + GetDescendantIds（供数据权限复用）
  user/         用户管理 + 数据权限过滤 + 改密/个人资料
  authn/        登录/登出/权限码（HTTP 路径仍是 /auth，包名区分 framework/auth）
  online/       在线用户列表 + 强制下线（基于 Redis session，无独立表）
  dict/         字典类型 + 字典数据
  config/       系统参数 key-value
  file/         文件上传/下载/删除
  file/storage/ 存储 SPI：Local/OSS/S3/SFTP/FTP 五个驱动 + Factory（缓存当前激活配置）
  fileconfig/   存储配置管理（密钥脱敏、编辑合并、激活切换）
  job/          定时任务：robfig/cron 调度器 + bean 注册表 + 执行日志
  syslog/       操作日志 + 登录日志的落库与查询
  ai/           AI 助手模块（复刻 fast-ai），下列子包共用 settings 读写 sys_config 的 ai.* 参数
    settings/   ai.* 运行期配置（助手开关/系统提示词/SQL 工具开关/RAG/MCP 等）的强类型读写
    config/     /ai/config 聚合配置读写（含密钥脱敏），对应 AiConfigController
    model/      ai_model_config CRUD + 连通性探测（fetch-models/test）+ 激活/启用
    tool/       ai_tool_config CRUD + SQL/HTTP 执行器（命名参数/行限制/敏感列脱敏）+ 内置工具 Spec
    rag/        知识库/文档/切片 CRUD + embedding + Qdrant + 文档抽取 + 切片 + 召回 + 异步索引
    mcp/        ai_mcp_server CRUD + 客户端管理器（stdio/sse/streamable-http）+ inspect + 保活
    agent/      对话 SSE + 手写工具调用循环（Anthropic/OpenAI）+ 历史 + 二次确认 + 审计 + 用量统计
internal/bootstrap/             组装全部模块 + 路由注册，对应启动类
```

## 已验证能力（对照现有 Java 项目逐项复刻）

- 登录：BCrypt 密码校验、账号状态校验、失败/成功都记登录日志，语义对齐 `AuthService.login`
- Token：多端并发登录（is-concurrent=true / is-share=false）、滑动续期、按 token 强制下线
- 权限：`/auth/codes` 返回去重权限码；菜单树按钮过滤 + 按 `meta_order` 排序，逻辑对齐 `buildMenuTree`
- 数据权限：用户列表按当前登录人角色实时计算 `全部/本部门及以下/本部门/自定义/仅本人` 并集过滤
- 审计字段：`created_by/created_id/updated_by/updated_id` 由 GORM 钩子从 context 里的当前登录人自动回填
- 操作日志：中间件捕获请求/响应体，四类敏感信息正则脱敏，响应体超 2000 字符截断，异步落库
- 文件存储：SPI + 工厂模式，密钥字段脱敏返回、编辑时未改的密钥自动保留原值，配置激活事件驱动缓存失效
- 定时任务：cron 表达式校验（6 段，含秒）、启动时恢复已启用任务、暂停/启动/立即执行一次

以上均在本机真实 MySQL + Redis 环境下跑通：登录 → 拿 token → 访问受保护接口 → 建字典/配置/任务 →
任务真实执行并写日志 → 本地文件上传/下载/物理落盘验证 → 删除 → 登出 → token 失效返回 401。

## 运行

```bash
# 需要本地 MySQL（用 scripts/sql/fast_admin_init.sql 建好库表）和 Redis
# 改 configs/config.yaml 或建一个 configs/config.<env>.yaml 覆盖连接串
go run ./cmd/server -env dev
```

`GET /health` 无鉴权；`/auth/login` 公开；其余路由都要求登录态，请求头带
`Authorization: <token>`（header 名可在 `configs/config.yaml` 的 `auth.token_header` 改）。

## 已知的简化 / 待完善点

- Excel 导入导出（用户批量导入等）未实现，需要接入 `excelize`
- 定时任务的 misfire 策略字段保留但未做差异化处理，robfig/cron 和 Quartz 的补偿语义不完全等价
- FTP/SFTP 驱动走通了基本上传/下载/删除，没有覆盖所有边缘情况（比如断点续传）
- 代码生成器（读 information_schema 生成六件套）未实现
- 演示模式拦截器（`DemoModeInterceptor`）未迁移，如需只读演示环境要自己加一个全局中间件
- AI 文档抽取支持 txt/md/csv/json/xml/html/yml、docx、pptx、xls/xlsx；旧版二进制 doc/ppt
  （OLE 复合文档）Go 侧无成熟解析库，暂不支持，会明确提示另存为新格式后再上传
- AI 的 `execute_sql` 因 Java 侧受演示模式约束，Go 侧未迁移演示模式，等价于 `demo.enabled=false`；
  执行前的用户二次确认仍保留（`/ai/agent/confirm/:token`）
- MCP 保活在 Go 侧用内部 ticker 实现（按各 SSE 服务的间隔 ping），不再往 `sys_job` 写保活任务；
  `ai_mcp_server.keep_alive_job_id` 因此保持为空
- MCP streamable-http 直接用官方 Go SDK，未实现 Java 侧的「SDK 失败再退化为手写 JSON-RPC」兜底
- 多轮工具调用的 token 用量取最后一轮（与 Java 取最后一帧 usage 的行为一致，跨轮不累加）
