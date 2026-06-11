// Package memory 提供 store.ConfigStore 与 store.Cache 的纯内存实现。
//
// 用途：离线自测、演示、单机零依赖运行。配合 observ 的内存 Sink，
// 整个网关可在没有 PG/ClickHouse/Redis/S3 的情况下端到端跑通。
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/claude-gate/claude-gate/internal/auth"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/store"
)

// ConfigStore 是内存版配置存储，实现 store.ConfigStore。
type ConfigStore struct {
	mu        sync.RWMutex
	users     map[int64]*domain.User
	channels  map[int64]*domain.UpstreamChannel
	upKeys    map[int64]*domain.UpstreamKey
	groups    map[int64]*domain.Group
	apiKeys   map[int64]*domain.APIKey
	mappings  map[int64]*domain.ModelMapping
	seq       map[string]int64
}

var _ store.ConfigStore = (*ConfigStore)(nil)

// NewConfigStore 构造空的内存配置存储。
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		users:    map[int64]*domain.User{},
		channels: map[int64]*domain.UpstreamChannel{},
		upKeys:   map[int64]*domain.UpstreamKey{},
		groups:   map[int64]*domain.Group{},
		apiKeys:  map[int64]*domain.APIKey{},
		mappings: map[int64]*domain.ModelMapping{},
		seq:      map[string]int64{},
	}
}

func (s *ConfigStore) nextID(kind string) int64 {
	s.seq[kind]++
	return s.seq[kind]
}

// ---- Users ----

func (s *ConfigStore) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *ConfigStore) ListUsers(_ context.Context) ([]domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *ConfigStore) CreateUser(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u.ID = s.nextID("user")
	u.CreatedAt = time.Now()
	cp := *u
	s.users[u.ID] = &cp
	return nil
}

// ---- Channels ----

func (s *ConfigStore) ListChannels(_ context.Context) ([]domain.UpstreamChannel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.UpstreamChannel, 0, len(s.channels))
	for _, c := range s.channels {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *ConfigStore) GetChannel(_ context.Context, id int64) (*domain.UpstreamChannel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.channels[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (s *ConfigStore) CreateChannel(_ context.Context, c *domain.UpstreamChannel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ID = s.nextID("channel")
	c.CreatedAt = time.Now()
	c.UpdatedAt = c.CreatedAt
	cp := *c
	s.channels[c.ID] = &cp
	return nil
}

func (s *ConfigStore) UpdateChannel(_ context.Context, c *domain.UpstreamChannel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.channels[c.ID]; !ok {
		return store.ErrNotFound
	}
	c.UpdatedAt = time.Now()
	cp := *c
	s.channels[c.ID] = &cp
	return nil
}

func (s *ConfigStore) DeleteChannel(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.channels, id)
	return nil
}

// ---- Upstream Keys ----

func (s *ConfigStore) ListUpstreamKeys(_ context.Context, channelID int64) ([]domain.UpstreamKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.UpstreamKey, 0)
	for _, k := range s.upKeys {
		if channelID == 0 || k.ChannelID == channelID {
			out = append(out, *k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *ConfigStore) CreateUpstreamKey(_ context.Context, k *domain.UpstreamKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k.ID = s.nextID("upkey")
	k.CreatedAt = time.Now()
	if k.Status == "" {
		k.Status = domain.KeyActive
	}
	cp := *k
	s.upKeys[k.ID] = &cp
	return nil
}

func (s *ConfigStore) UpdateUpstreamKey(_ context.Context, k *domain.UpstreamKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.upKeys[k.ID]; !ok {
		return store.ErrNotFound
	}
	cp := *k
	s.upKeys[k.ID] = &cp
	return nil
}

func (s *ConfigStore) DeleteUpstreamKey(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.upKeys, id)
	return nil
}

// ---- Groups ----

func (s *ConfigStore) ListGroups(_ context.Context) ([]domain.Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *ConfigStore) GetGroup(_ context.Context, id int64) (*domain.Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *g
	return &cp, nil
}

func (s *ConfigStore) CreateGroup(_ context.Context, g *domain.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g.ID = s.nextID("group")
	g.CreatedAt = time.Now()
	g.UpdatedAt = g.CreatedAt
	cp := *g
	s.groups[g.ID] = &cp
	return nil
}

func (s *ConfigStore) UpdateGroup(_ context.Context, g *domain.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[g.ID]; !ok {
		return store.ErrNotFound
	}
	g.UpdatedAt = time.Now()
	cp := *g
	s.groups[g.ID] = &cp
	return nil
}

func (s *ConfigStore) DeleteGroup(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.groups, id)
	return nil
}

// ---- API Keys ----

func (s *ConfigStore) ListAPIKeys(_ context.Context) ([]domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.APIKey, 0, len(s.apiKeys))
	for _, k := range s.apiKeys {
		out = append(out, *k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *ConfigStore) GetAPIKey(_ context.Context, id int64) (*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.apiKeys[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *k
	return &cp, nil
}

func (s *ConfigStore) GetAPIKeyByPrefix(_ context.Context, prefix string) (*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.apiKeys {
		if k.KeyPrefix == prefix {
			cp := *k
			return &cp, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *ConfigStore) CreateAPIKey(_ context.Context, k *domain.APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k.ID = s.nextID("apikey")
	k.CreatedAt = time.Now()
	cp := *k
	s.apiKeys[k.ID] = &cp
	return nil
}

func (s *ConfigStore) UpdateAPIKey(_ context.Context, k *domain.APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.apiKeys[k.ID]; !ok {
		return store.ErrNotFound
	}
	cp := *k
	s.apiKeys[k.ID] = &cp
	return nil
}

func (s *ConfigStore) DeleteAPIKey(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.apiKeys, id)
	return nil
}

// ---- Model Mappings ----

func (s *ConfigStore) ListModelMappings(_ context.Context, channelID int64) ([]domain.ModelMapping, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.ModelMapping, 0)
	for _, m := range s.mappings {
		if channelID == 0 || m.ChannelID == channelID {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *ConfigStore) CreateModelMapping(_ context.Context, m *domain.ModelMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.ID = s.nextID("mapping")
	cp := *m
	s.mappings[m.ID] = &cp
	return nil
}

func (s *ConfigStore) DeleteModelMapping(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mappings, id)
	return nil
}

// SelfTestAPIKey 是种子数据里固定的客户 Key 明文，供离线自测直接调用 /v1/messages。
const SelfTestAPIKey = "cg-selftest-deadbeefdeadbeefdeadbeef"

// Seed 写入一套演示种子数据，并返回内存配置存储。
//
// 包含一个绑定到本地 mock（custom）通道的分组与固定明文的客户 Key，
// 使网关可离线端到端跑通。
func (s *ConfigStore) Seed(encKey string) { SeedConfigStore(context.Background(), s, encKey) }

// SeedConfigStore 向任意 ConfigStore 写入一套演示种子数据（内存与真实库通用）。
func SeedConfigStore(ctx context.Context, s store.ConfigStore, encKey string) {
	_ = s.CreateUser(ctx, &domain.User{Email: "admin@claude-gate.io", PasswordHash: auth.HashSecret("admin123"), Role: "admin"})

	mockCh := &domain.UpstreamChannel{Name: "本地 Mock 通道", Type: domain.ChannelCustom, Enabled: true, Config: map[string]any{}}
	_ = s.CreateChannel(ctx, mockCh)
	official := &domain.UpstreamChannel{Name: "Anthropic 官方", Type: domain.ChannelOfficial, BaseURL: "https://api.anthropic.com", Enabled: true, Config: map[string]any{"anthropic_version": "2023-06-01"}}
	_ = s.CreateChannel(ctx, official)
	kiroCh := &domain.UpstreamChannel{Name: "Kiro 主通道（透传）", Type: domain.ChannelKiro, BaseURL: "https://prod.kiro.internal", Enabled: true, Config: map[string]any{}}
	_ = s.CreateChannel(ctx, kiroCh)
	relayCh := &domain.UpstreamChannel{Name: "第三方中转 A", Type: domain.ChannelRelay, BaseURL: "https://relay-a.example.com", Enabled: true, Config: map[string]any{"auth_mode": "bearer"}}
	_ = s.CreateChannel(ctx, relayCh)

	_ = s.CreateUpstreamKey(ctx, &domain.UpstreamKey{ChannelID: mockCh.ID, Name: "mock-key", Status: domain.KeyActive, CredentialEncrypted: "mock"})
	_ = s.CreateUpstreamKey(ctx, &domain.UpstreamKey{ChannelID: official.ID, Name: "official-key-01", Status: domain.KeyActive, CredentialEncrypted: "sk-ant-xxx"})
	_ = s.CreateUpstreamKey(ctx, &domain.UpstreamKey{ChannelID: official.ID, Name: "official-key-02", Status: domain.KeyActive, CredentialEncrypted: "sk-ant-yyy"})
	_ = s.CreateUpstreamKey(ctx, &domain.UpstreamKey{ChannelID: kiroCh.ID, Name: "kiro-pool-01", Status: domain.KeyActive, CredentialEncrypted: "kiro-key-1"})
	_ = s.CreateUpstreamKey(ctx, &domain.UpstreamKey{ChannelID: kiroCh.ID, Name: "kiro-pool-02", Status: domain.KeyDisabled, CredentialEncrypted: "kiro-key-2"})
	_ = s.CreateUpstreamKey(ctx, &domain.UpstreamKey{ChannelID: relayCh.ID, Name: "relay-key-01", Status: domain.KeyActive, CredentialEncrypted: "relay-key"})

	// 四种缓存策略各一个分组
	passthrough := &domain.Group{Name: "默认-透传", Description: "本地 mock 通道，usage 透传", ChannelID: mockCh.ID, Enabled: true,
		CacheStrategy:     domain.CacheStrategyConfig{Type: "passthrough"},
		TransformerConfig: []domain.TransformerConfig{{Name: "streaming_event_fixer", Enabled: true}},
		RateLimit:         domain.RateLimitConfig{RPM: 600, TPM: 800000}}
	_ = s.CreateGroup(ctx, passthrough)
	_ = s.CreateGroup(ctx, &domain.Group{Name: "Kiro-百分比", Description: "按上下文比例计 cache", ChannelID: kiroCh.ID, Enabled: true,
		CacheStrategy:     domain.CacheStrategyConfig{Type: "percentage", Params: map[string]any{"cache_creation_ratio": 0.1, "cache_read_ratio": 0.9, "input_fixed_tokens": 1, "output_source": "upstream"}},
		TransformerConfig: []domain.TransformerConfig{{Name: "tool_call_normalizer", Enabled: true}}})
	// 固定值分组绑定本地 mock 通道，便于离线演示/复现"非透传"计费改写
	_ = s.CreateGroup(ctx, &domain.Group{Name: "基础-固定值", Description: "固定计费（本地 mock 通道）", ChannelID: mockCh.ID, Enabled: true,
		CacheStrategy: domain.CacheStrategyConfig{Type: "fixed", Params: map[string]any{"input_tokens": 1, "output_tokens": 0, "cache_creation_tokens": 1000, "cache_read_tokens": 5000}}})
	_ = s.CreateGroup(ctx, &domain.Group{Name: "公式计费", Description: "公式引擎自定义口径", ChannelID: official.ID, Enabled: true,
		CacheStrategy: domain.CacheStrategyConfig{Type: "formula", Params: map[string]any{"input": "1", "cache_creation": "total * 0.1", "cache_read": "total - cache_creation - input", "output": "upstream_output"}}})

	// 固定明文的自测 Key，绑定到透传分组（本地 mock 通道）
	enc, _ := auth.Encrypt(SelfTestAPIKey, encKey)
	_ = s.CreateAPIKey(ctx, &domain.APIKey{KeyPrefix: "selftest", KeyHash: auth.HashSecret("deadbeefdeadbeefdeadbeef"), KeyEncrypted: enc, Name: "自测 Key", GroupID: passthrough.ID, Enabled: true})

	_ = s.CreateModelMapping(ctx, &domain.ModelMapping{ChannelID: kiroCh.ID, ClientModel: "claude-sonnet-4-20250514", UpstreamModel: "kiro-claude-sonnet-4"})
}
