// Package config 负责加载 claude-gate 的运行配置。
//
// 配置来源优先级：默认值 < YAML 文件 < 环境变量（前缀 CG_）。
// 所有可调参数都集中在此，禁止散落在各模块硬编码（任务书 §10）。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是顶层配置。
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Postgres    DSNConfig         `yaml:"postgres"`
	ClickHouse  DSNConfig         `yaml:"clickhouse"`
	Redis       RedisConfig       `yaml:"redis"`
	S3          S3Config          `yaml:"s3"`
	Concurrency ConcurrencyConfig `yaml:"concurrency"`
	Sampling    SamplingConfig    `yaml:"sampling"`
	Auth        AuthConfig        `yaml:"auth"`
	Log         LogConfig         `yaml:"log"`
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Addr            string `yaml:"addr"`
	RequestTimeout  int    `yaml:"request_timeout_seconds"`  // 全链路超时，默认 600s
	ShutdownTimeout int    `yaml:"shutdown_timeout_seconds"` // 优雅退出超时
}

// DSNConfig 通用数据库连接配置。
type DSNConfig struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// RedisConfig Redis 配置。
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// S3Config 对象存储配置。
type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
}

// ConcurrencyConfig 并发与连接池上限（任务书 §2.1 性能约束）。
type ConcurrencyConfig struct {
	GlobalMaxInFlight  int `yaml:"global_max_in_flight"`  // 全局并发请求上限，超出快速返回 429
	PerChannelInFlight int `yaml:"per_channel_in_flight"` // 每通道并发上限
	WorkerPoolSize     int `yaml:"worker_pool_size"`      // 异步落盘 worker 数
	UpstreamPoolSize   int `yaml:"upstream_pool_size"`    // 每上游 HTTP 连接池上限
}

// SamplingConfig 落盘采样配置（任务书 §5.6）。
type SamplingConfig struct {
	SuccessRate  float64 `yaml:"success_rate"`  // 成功请求落盘采样率，默认 0.01
	ErrorAlways  bool    `yaml:"error_always"`  // 错误请求是否 100% 落盘，默认 true
	S3WriteRetry int     `yaml:"s3_write_retry"` // S3 写入失败重试次数
}

// AuthConfig 管理后台鉴权配置。
type AuthConfig struct {
	JWTSecret     string `yaml:"jwt_secret"`
	JWTTTLMinutes int    `yaml:"jwt_ttl_minutes"`
	APIKeyTTLSec  int    `yaml:"apikey_cache_ttl_seconds"` // API Key → Group 缓存秒数，默认 60
	// EncryptionKey 对称加密口令，用于可逆存储客户 API Key 明文与上游凭证，
	// 支持管理后台"重复查看"客户 Key（见 internal/auth/crypto.go）。生产必须配置。
	EncryptionKey string `yaml:"encryption_key"`
}

// LogConfig 日志配置。
type LogConfig struct {
	Level  string `yaml:"level"`  // debug / info / warn / error
	Format string `yaml:"format"` // json / text
}

// Default 返回内置默认配置，对应任务书推荐基线（16 核 / 32 GB）。
func Default() Config {
	return Config{
		Server: ServerConfig{
			Addr:            ":8791",
			RequestTimeout:  600,
			ShutdownTimeout: 30,
		},
		Postgres:   DSNConfig{DSN: "postgres://claude:claude@localhost:5432/claude_gate?sslmode=disable", MaxOpenConns: 20, MaxIdleConns: 10},
		ClickHouse: DSNConfig{DSN: "clickhouse://localhost:9000/claude_gate", MaxOpenConns: 10, MaxIdleConns: 5},
		Redis:      RedisConfig{Addr: "localhost:6379", DB: 0, PoolSize: 50},
		S3:         S3Config{Endpoint: "localhost:9000", Region: "us-east-1", Bucket: "claude-gate", UseSSL: false},
		Concurrency: ConcurrencyConfig{
			GlobalMaxInFlight:  6000,
			PerChannelInFlight: 2000,
			WorkerPoolSize:     16,
			UpstreamPoolSize:   256,
		},
		Sampling: SamplingConfig{SuccessRate: 0.01, ErrorAlways: true, S3WriteRetry: 3},
		Auth:     AuthConfig{JWTTTLMinutes: 720, APIKeyTTLSec: 60},
		Log:      LogConfig{Level: "info", Format: "json"},
	}
}

// Load 从 YAML 文件加载配置并以默认值兜底，最后用环境变量覆盖。
// path 为空时只用默认值 + 环境变量。
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("读取配置文件失败: %w", err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("解析配置文件失败: %w", err)
		}
	}
	applyEnv(&cfg)
	return cfg, nil
}

// applyEnv 用环境变量覆盖关键配置项（前缀 CG_）。
func applyEnv(cfg *Config) {
	if v := os.Getenv("CG_SERVER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("CG_POSTGRES_DSN"); v != "" {
		cfg.Postgres.DSN = v
	}
	if v := os.Getenv("CG_CLICKHOUSE_DSN"); v != "" {
		cfg.ClickHouse.DSN = v
	}
	if v := os.Getenv("CG_REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("CG_S3_ENDPOINT"); v != "" {
		cfg.S3.Endpoint = v
	}
	if v := os.Getenv("CG_S3_BUCKET"); v != "" {
		cfg.S3.Bucket = v
	}
	if v := os.Getenv("CG_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("CG_ENCRYPTION_KEY"); v != "" {
		cfg.Auth.EncryptionKey = v
	}
	if v := os.Getenv("CG_LOG_LEVEL"); v != "" {
		cfg.Log.Level = strings.ToLower(v)
	}
	if v := os.Getenv("CG_GLOBAL_MAX_IN_FLIGHT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Concurrency.GlobalMaxInFlight = n
		}
	}
}
