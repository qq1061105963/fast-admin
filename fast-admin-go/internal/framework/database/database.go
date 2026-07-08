// Package database 负责多数据源的 GORM 初始化，对应 fast-framework/database。
package database

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/config"
)

// Manager 持有所有已初始化的数据源，primary 是默认业务库。
type Manager struct {
	primary string
	sources map[string]*gorm.DB
}

// NewManager 根据配置初始化全部数据源；某个数据源初始化失败会立即返回错误，
// 避免应用带着一个坏掉的数据源静默启动。
func NewManager(cfg config.DatabaseConfig) (*Manager, error) {
	if cfg.Primary == "" {
		return nil, fmt.Errorf("database.primary is required")
	}
	m := &Manager{primary: cfg.Primary, sources: make(map[string]*gorm.DB, len(cfg.Sources))}

	for name, ds := range cfg.Sources {
		db, err := open(ds)
		if err != nil {
			return nil, fmt.Errorf("open datasource %q: %w", name, err)
		}
		m.sources[name] = db
	}
	if _, ok := m.sources[cfg.Primary]; !ok {
		return nil, fmt.Errorf("primary datasource %q not found in database.sources", cfg.Primary)
	}
	return m, nil
}

func open(ds config.DataSourceConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch ds.Driver {
	case "mysql":
		dialector = mysql.Open(ds.DSN)
	case "postgres":
		dialector = postgres.Open(ds.DSN)
	default:
		return nil, fmt.Errorf("unsupported driver %q (expect mysql/postgres)", ds.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Warn)})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if ds.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(ds.MaxOpenConns)
	}
	if ds.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(ds.MaxIdleConns)
	}
	if ds.ConnMaxLifeMins > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(ds.ConnMaxLifeMins) * time.Minute)
	}
	return db, nil
}

// DB 返回默认（primary）数据源，绝大多数业务模块只需要这一个。
func (m *Manager) DB() *gorm.DB {
	return m.sources[m.primary]
}

// Named 返回指定名字的数据源，用于跨库查询等少数场景。
func (m *Manager) Named(name string) (*gorm.DB, bool) {
	db, ok := m.sources[name]
	return db, ok
}

// Close 关闭所有数据源连接，在应用退出时调用。
func (m *Manager) Close() error {
	for name, db := range m.sources {
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("get sql.DB for %q: %w", name, err)
		}
		if err := sqlDB.Close(); err != nil {
			return fmt.Errorf("close datasource %q: %w", name, err)
		}
	}
	return nil
}
