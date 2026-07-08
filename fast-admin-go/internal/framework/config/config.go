// Package config 负责加载多 profile 配置文件，对应 Java 侧的
// application.yml + application-{profile}.yml 组织方式。
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 是应用的全量配置结构，按现有 Java 项目的 profile 拆分方式，
// 每个子配置对应一个 application-xxx.yml。
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Upload   UploadConfig   `mapstructure:"upload"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug/release/test，对应 gin.Mode
}

type LogConfig struct {
	Level string `mapstructure:"level"` // debug/info/warn/error
	Path  string `mapstructure:"path"`
}

// DataSourceConfig 描述单个数据源，Driver 支持 mysql/postgres，
// 对应现在 pom.xml 里同时管理 mysql-connector-j 与 postgresql 驱动。
type DataSourceConfig struct {
	Driver          string `mapstructure:"driver"`
	DSN             string `mapstructure:"dsn"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifeMins int    `mapstructure:"conn_max_life_mins"`
}

// DatabaseConfig 支持一个 primary 数据源 + 任意数量的具名数据源，
// 对应 fast-framework/database 里的多数据源能力。
type DatabaseConfig struct {
	Primary string                      `mapstructure:"primary"`
	Sources map[string]DataSourceConfig `mapstructure:"sources"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// AuthConfig 对应替代 Sa-Token 的 token/session 配置。
type AuthConfig struct {
	TokenHeader   string `mapstructure:"token_header"`
	TokenTTLHours int    `mapstructure:"token_ttl_hours"`
	TokenPrefix   string `mapstructure:"token_prefix"` // redis key 前缀
}

type UploadConfig struct {
	Driver string `mapstructure:"driver"` // local/oss/s3/sftp
	Local  struct {
		Dir string `mapstructure:"dir"`
	} `mapstructure:"local"`
}

// Load 按 base + profile 覆盖的方式加载配置：
// 先读 configs/config.yaml，再用 configs/config.{env}.yaml 覆盖同名字段，
// env 优先取入参，其次取 APP_ENV 环境变量，默认 dev。
func Load(configDir, env string) (*Config, error) {
	if env == "" {
		env = os.Getenv("APP_ENV")
	}
	if env == "" {
		env = "dev"
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read base config: %w", err)
	}

	overlay := viper.New()
	overlay.SetConfigName("config." + env)
	overlay.SetConfigType("yaml")
	overlay.AddConfigPath(configDir)
	if err := overlay.ReadInConfig(); err == nil {
		if mergeErr := v.MergeConfigMap(overlay.AllSettings()); mergeErr != nil {
			return nil, fmt.Errorf("merge %s config: %w", env, mergeErr)
		}
	} else if !strings.Contains(err.Error(), "Not Found") {
		return nil, fmt.Errorf("read %s config: %w", env, err)
	}

	// 支持 FAST_ADMIN_SERVER_PORT 这类环境变量覆盖。
	v.SetEnvPrefix("FAST_ADMIN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
