// Package bootstrap 负责把 config/logger/database/redis/router 组装成一个可运行的 App，
// 对应 fast-application 里 Spring Boot 启动类做的事情。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/auth"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/config"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/database"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/middleware"

	aiagent "github.com/SirYuxuan/fast-admin-go/internal/modules/ai/agent"
	aiconfig "github.com/SirYuxuan/fast-admin-go/internal/modules/ai/config"
	aimcp "github.com/SirYuxuan/fast-admin-go/internal/modules/ai/mcp"
	aimodel "github.com/SirYuxuan/fast-admin-go/internal/modules/ai/model"
	airag "github.com/SirYuxuan/fast-admin-go/internal/modules/ai/rag"
	aisettings "github.com/SirYuxuan/fast-admin-go/internal/modules/ai/settings"
	aitool "github.com/SirYuxuan/fast-admin-go/internal/modules/ai/tool"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/authn"
	sysconfig "github.com/SirYuxuan/fast-admin-go/internal/modules/config"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/dept"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/dict"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/file"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/file/storage"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/fileconfig"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/job"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/menu"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/online"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/permission"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/role"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/syslog"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/user"
)

type App struct {
	cfg       *config.Config
	db        *database.Manager
	rdb       *redis.Client
	tokens    *auth.TokenService
	router    *gin.Engine
	server    *http.Server
	scheduler *job.Scheduler
	mcpMgr    *aimcp.Manager
}

// New 按顺序初始化 logger -> database -> redis -> auth -> 各业务模块 -> router。
// 任何一步失败都直接返回错误，不允许应用带着半初始化状态启动。
func New(configDir, env string) (*App, error) {
	cfg, err := config.Load(configDir, env)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	if err := logger.Init(cfg.Log); err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	dbManager, err := database.NewManager(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}
	db := dbManager.DB()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	tokenService := auth.NewTokenService(rdb, cfg.Auth)

	// ---- 日志模块先起来，后面所有模块的操作日志/登录日志都要用它 ----
	logSvc := syslog.NewService(syslog.NewOperationLogRepository(db), syslog.NewLoginLogRepository(db))
	var opWriter = logSvc // 实现了 oplog.Writer.Save

	// ---- 身份权限核心：permission -> menu/dept/role -> user ----
	permRepo := permission.NewRepository(db)

	menuRepo := menu.NewRepository(db)
	menuSvc := menu.NewService(menuRepo, permRepo)

	deptRepo := dept.NewRepository(db)
	deptSvc := dept.NewService(deptRepo)

	roleRepo := role.NewRepository(db)
	roleDeptRepo := role.NewDeptRepository(db)
	roleSvc := role.NewService(roleRepo, roleDeptRepo, permRepo)

	userRepo := user.NewRepository(db)
	userSvc := user.NewService(userRepo, permRepo, roleRepo, roleDeptRepo, deptSvc, tokenService)

	authnSvc := authn.NewService(userRepo, permRepo, menuSvc, tokenService, logSvc.AsLoginLogWriter())

	onlineSvc := online.NewService(tokenService)

	// ---- 常规业务模块 ----
	dictSvc := dict.NewService(dict.NewTypeRepository(db), dict.NewDataRepository(db))
	configSvc := sysconfig.NewService(sysconfig.NewRepository(db))

	// ---- 文件存储：factory 先用 nil provider 构造，等 fileconfig.Service 就绪后回填 ----
	storageFactory := storage.NewFactory(storage.NewRegistry())
	fileConfigRepo := fileconfig.NewRepository(db)
	fileConfigSvc := fileconfig.NewService(fileConfigRepo, storageFactory)
	storageFactory.SetProvider(fileConfigSvc)

	fileRepo := file.NewRepository(db)
	fileSvc := file.NewService(fileRepo, fileConfigRepo, storageFactory)
	fileConfigSvc.SetFileReferenceCounter(fileRepo)

	// ---- 定时任务：注册内置示例任务，启动调度器，恢复已启用的任务 ----
	jobRegistry := job.NewRegistry()
	registerBuiltinJobs(jobRegistry)
	jobLogRepo := job.NewLogRepository(db)
	scheduler := job.NewScheduler(jobRegistry, jobLogRepo)
	jobSvc := job.NewService(job.NewRepository(db), jobLogRepo, scheduler)
	scheduler.Start()
	if err := jobSvc.Bootstrap(context.Background()); err != nil {
		return nil, fmt.Errorf("bootstrap scheduled jobs: %w", err)
	}

	// ---- AI 模块（fast-ai 的 Go 复刻）：settings -> model/tool/rag/mcp -> agent ----
	aiSettings := aisettings.New(aisettings.NewStore(db))
	aiConfigSvc := aiconfig.NewService(aiSettings)

	aiModelSvc := aimodel.NewService(aimodel.NewRepository(db))

	aiToolRepo := aitool.NewRepository(db)
	aiToolSvc := aitool.NewService(aiToolRepo, aiSettings)
	aiToolExec := aitool.NewExecutor(aiToolRepo, db)

	aiRagSvc := airag.NewService(airag.NewRepository(db), airag.NewEmbedding(aiSettings),
		airag.NewQdrant(aiSettings), aiSettings, fileSvc)
	fileSvc.AddChecker(aiRagSvc) // 知识库引用的源文件禁止在文件管理里直接删除

	aiMcpSvc := aimcp.NewService(aimcp.NewRepository(db))
	aiMcpMgr := aimcp.NewManager(aiMcpSvc, aiSettings)
	aiConfigSvc.SetMCPReloader(aiMcpMgr)
	go aiMcpMgr.Reload() // 异步初始化 MCP 连接，失败不阻塞启动

	aiAgentRepo := aiagent.NewRepository(db)
	aiAgentSvc := aiagent.NewService(aiSettings, aiModelSvc, aiToolSvc, aiToolExec, aiMcpMgr, aiRagSvc,
		aiagent.NewHistoryService(aiAgentRepo, aiSettings), aiagent.NewConfirmationService(),
		aiagent.NewAuditLogger(aiAgentRepo), aiagent.NewStatsService(aiAgentRepo))

	gin.SetMode(modeOrDefault(cfg.Server.Mode))
	router := gin.New()
	router.Use(middleware.TraceID(), middleware.Recovery(), middleware.RequestLogger(), middleware.CORS())

	app := &App{cfg: cfg, db: dbManager, rdb: rdb, tokens: tokenService, router: router, scheduler: scheduler, mcpMgr: aiMcpMgr}

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	public := router.Group("")
	protected := router.Group("")
	protected.Use(middleware.Auth(tokenService, cfg.Auth.TokenHeader))

	authn.RegisterRoutes(public, protected, authn.NewHandler(authnSvc, cfg.Auth.TokenHeader), opWriter)
	menu.RegisterRoutes(protected, menu.NewHandler(menuSvc), opWriter)
	role.RegisterRoutes(protected, role.NewHandler(roleSvc), opWriter)
	user.RegisterRoutes(protected, user.NewHandler(userSvc), opWriter)
	dept.RegisterRoutes(protected, dept.NewHandler(deptSvc), opWriter)
	online.RegisterRoutes(protected, online.NewHandler(onlineSvc), opWriter)
	dict.RegisterRoutes(protected, dict.NewHandler(dictSvc), opWriter)
	sysconfig.RegisterRoutes(protected, sysconfig.NewHandler(configSvc), opWriter)
	fileconfig.RegisterRoutes(protected, fileconfig.NewHandler(fileConfigSvc), opWriter)
	file.RegisterRoutes(protected, file.NewHandler(fileSvc), opWriter)
	job.RegisterRoutes(protected, job.NewHandler(jobSvc), opWriter)
	syslog.RegisterRoutes(protected, syslog.NewHandler(logSvc))

	// ---- AI 模块路由 ----
	aiconfig.RegisterRoutes(protected, aiconfig.NewHandler(aiConfigSvc), opWriter)
	aimodel.RegisterRoutes(protected, aimodel.NewHandler(aiModelSvc), opWriter)
	aitool.RegisterRoutes(protected, aitool.NewHandler(aiToolSvc), opWriter)
	airag.RegisterRoutes(protected, airag.NewHandler(aiRagSvc), opWriter)
	aimcp.RegisterRoutes(protected, aimcp.NewHandler(aiMcpSvc, aiMcpMgr), opWriter)
	aiagent.RegisterRoutes(protected, aiagent.NewHandler(aiAgentSvc, aiAgentRepo))

	app.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return app, nil
}

// registerBuiltinJobs 注册可供定时任务调用的示例函数，对应 Java 侧 sys_job/example/DemoJob。
func registerBuiltinJobs(registry *job.Registry) {
	registry.Register("demoJob", func(ctx context.Context, params string) error {
		logger.L().Sugar().Infof("demoJob executed, params=%q", params)
		return nil
	})
	registry.Register("demoFailJob", func(ctx context.Context, params string) error {
		return errors.New("demo job intentionally failed")
	})
}

func modeOrDefault(mode string) string {
	if mode == "" {
		return gin.DebugMode
	}
	return mode
}

// Run 启动 HTTP 服务并阻塞，直到 ctx 被取消后执行优雅关闭。
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		logger.L().Sugar().Infof("server listening on %s", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	a.scheduler.Stop()
	if a.mcpMgr != nil {
		a.mcpMgr.Close()
	}
	if err := a.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	if err := a.rdb.Close(); err != nil {
		return fmt.Errorf("close redis: %w", err)
	}
	logger.Sync()
	return nil
}
