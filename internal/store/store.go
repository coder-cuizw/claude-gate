// Package store 定义各存储后端的抽象接口（任务书 §8）。
//
// 具体实现分布在子包：
//   - postgres：配置数据（users / channels / groups / api_keys …）
//   - clickhouse：请求明细与聚合查询
//   - redis：API Key → Group 解析缓存、限流
//   - s3：请求/响应 body 落盘
//
// 所有外部依赖均通过接口注入，便于用 mock 或 testcontainers 测试（任务书 §10）。
package store

import (
	"context"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// ConfigStore 是配置数据（PostgreSQL）的读写接口（节选核心方法）。
type ConfigStore interface {
	ListChannels(ctx context.Context) ([]domain.UpstreamChannel, error)
	GetGroup(ctx context.Context, id int64) (*domain.Group, error)
	ListGroups(ctx context.Context) ([]domain.Group, error)
	ListAPIKeys(ctx context.Context) ([]domain.APIKey, error)
	ListUpstreamKeys(ctx context.Context, channelID int64) ([]domain.UpstreamKey, error)
}

// Cache 是热路径缓存（Redis）接口。
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttlSeconds int) error
	Del(ctx context.Context, keys ...string) error
}
