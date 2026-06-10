// Package store 定义各存储后端的抽象接口（任务书 §8）。
//
// 具体实现：
//   - memory：纯内存实现，用于离线自测、演示与单机零依赖运行
//   - postgres：配置数据（PostgreSQL）
//   - redis：API Key → Group 解析缓存、限流
//
// 明细与 body 落盘见 internal/observ（Sink / BodyStore / MetricsReader）。
// 所有外部依赖均通过接口注入，便于用 mock 或 testcontainers 测试（任务书 §10）。
package store

import (
	"context"
	"errors"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("store: 记录不存在")

// ConfigStore 是配置数据（PostgreSQL）的读写接口，覆盖管理后台所需的全部 CRUD。
type ConfigStore interface {
	// Users
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	CreateUser(ctx context.Context, u *domain.User) error

	// Channels
	ListChannels(ctx context.Context) ([]domain.UpstreamChannel, error)
	GetChannel(ctx context.Context, id int64) (*domain.UpstreamChannel, error)
	CreateChannel(ctx context.Context, c *domain.UpstreamChannel) error
	UpdateChannel(ctx context.Context, c *domain.UpstreamChannel) error
	DeleteChannel(ctx context.Context, id int64) error

	// Upstream Keys
	ListUpstreamKeys(ctx context.Context, channelID int64) ([]domain.UpstreamKey, error)
	CreateUpstreamKey(ctx context.Context, k *domain.UpstreamKey) error
	UpdateUpstreamKey(ctx context.Context, k *domain.UpstreamKey) error
	DeleteUpstreamKey(ctx context.Context, id int64) error

	// Groups
	ListGroups(ctx context.Context) ([]domain.Group, error)
	GetGroup(ctx context.Context, id int64) (*domain.Group, error)
	CreateGroup(ctx context.Context, g *domain.Group) error
	UpdateGroup(ctx context.Context, g *domain.Group) error
	DeleteGroup(ctx context.Context, id int64) error

	// API Keys
	ListAPIKeys(ctx context.Context) ([]domain.APIKey, error)
	GetAPIKey(ctx context.Context, id int64) (*domain.APIKey, error)
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error)
	CreateAPIKey(ctx context.Context, k *domain.APIKey) error
	UpdateAPIKey(ctx context.Context, k *domain.APIKey) error
	DeleteAPIKey(ctx context.Context, id int64) error

	// Model Mappings
	ListModelMappings(ctx context.Context, channelID int64) ([]domain.ModelMapping, error)
	CreateModelMapping(ctx context.Context, m *domain.ModelMapping) error
	DeleteModelMapping(ctx context.Context, id int64) error
}

// Cache 是热路径缓存（Redis）接口。
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttlSeconds int) error
	Del(ctx context.Context, keys ...string) error
}
