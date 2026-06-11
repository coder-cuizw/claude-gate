// Package postgres 用 pgx 实现 store.ConfigStore（配置数据真实落库）。
//
// 所有 JSONB 字段（通道 config、分组 cache_strategy/transformer/限流/重试）
// 以 json 编解码进出；可空文本列在 SQL 内用 COALESCE 收敛为非空字符串。
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/store"
)

// Store 是基于 PostgreSQL 的配置存储。
type Store struct {
	pool *pgxpool.Pool
}

var _ store.ConfigStore = (*Store)(nil)

// New 连接 PG 并返回存储。dsn 形如 postgres://user:pass@host:5432/db?sslmode=disable。
func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析 PG DSN 失败: %w", err)
	}
	// 连接池上限可由 DSN 的 pool_max_conns 控制；这里给一个稳健默认
	if cfg.MaxConns == 0 {
		cfg.MaxConns = 20
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("建立 PG 连接池失败: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("PG 连接探活失败: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close 关闭连接池。
func (s *Store) Close() { s.pool.Close() }

// Ping 供 readyz 探活使用。
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	return err
}

// ---- Users ----

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE email=$1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, email, password_hash, role, created_at, updated_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, u *domain.User) error {
	if u.Role == "" {
		u.Role = "admin"
	}
	return s.pool.QueryRow(ctx,
		`INSERT INTO users(email, password_hash, role) VALUES($1,$2,$3) RETURNING id, created_at, updated_at`,
		u.Email, u.PasswordHash, u.Role,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (s *Store) UpdateUserPassword(ctx context.Context, userID int64, newHash string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2`,
		newHash, userID,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("用户不存在: id=%d", userID)
	}
	return nil
}

// ---- Channels ----

func (s *Store) ListChannels(ctx context.Context) ([]domain.UpstreamChannel, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, type, COALESCE(base_url,''), config, enabled, created_at, updated_at FROM upstream_channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.UpstreamChannel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) GetChannel(ctx context.Context, id int64) (*domain.UpstreamChannel, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, type, COALESCE(base_url,''), config, enabled, created_at, updated_at FROM upstream_channels WHERE id=$1`, id)
	c, err := scanChannel(row)
	if err != nil {
		return nil, mapErr(err)
	}
	return c, nil
}

func (s *Store) CreateChannel(ctx context.Context, c *domain.UpstreamChannel) error {
	cfg := jsonbOf(c.Config)
	return s.pool.QueryRow(ctx,
		`INSERT INTO upstream_channels(name, type, base_url, config, enabled) VALUES($1,$2,$3,$4,$5)
		 RETURNING id, created_at, updated_at`,
		c.Name, string(c.Type), c.BaseURL, cfg, c.Enabled,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (s *Store) UpdateChannel(ctx context.Context, c *domain.UpstreamChannel) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE upstream_channels SET name=$2, type=$3, base_url=$4, config=$5, enabled=$6, updated_at=NOW() WHERE id=$1`,
		c.ID, c.Name, string(c.Type), c.BaseURL, jsonbOf(c.Config), c.Enabled)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteChannel(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM upstream_channels WHERE id=$1`, id)
	return err
}

// ---- Upstream Keys ----

func (s *Store) ListUpstreamKeys(ctx context.Context, channelID int64) ([]domain.UpstreamKey, error) {
	q := `SELECT id, channel_id, COALESCE(name,''), credential_encrypted, status, COALESCE(last_error,''), last_used_at, created_at
	      FROM upstream_keys`
	args := []any{}
	if channelID != 0 {
		q += ` WHERE channel_id=$1`
		args = append(args, channelID)
	}
	q += ` ORDER BY id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.UpstreamKey
	for rows.Next() {
		var k domain.UpstreamKey
		var status string
		if err := rows.Scan(&k.ID, &k.ChannelID, &k.Name, &k.CredentialEncrypted, &status, &k.LastError, &k.LastUsedAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		k.Status = domain.KeyStatus(status)
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) CreateUpstreamKey(ctx context.Context, k *domain.UpstreamKey) error {
	if k.Status == "" {
		k.Status = domain.KeyActive
	}
	return s.pool.QueryRow(ctx,
		`INSERT INTO upstream_keys(channel_id, name, credential_encrypted, status) VALUES($1,$2,$3,$4)
		 RETURNING id, created_at`,
		k.ChannelID, k.Name, k.CredentialEncrypted, string(k.Status),
	).Scan(&k.ID, &k.CreatedAt)
}

func (s *Store) UpdateUpstreamKey(ctx context.Context, k *domain.UpstreamKey) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE upstream_keys SET name=$2, credential_encrypted=$3, status=$4, last_error=NULLIF($5,''), last_used_at=$6 WHERE id=$1`,
		k.ID, k.Name, k.CredentialEncrypted, string(k.Status), k.LastError, k.LastUsedAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteUpstreamKey(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM upstream_keys WHERE id=$1`, id)
	return err
}

// ---- Groups ----

func (s *Store) ListGroups(ctx context.Context) ([]domain.Group, error) {
	rows, err := s.pool.Query(ctx, groupSelect+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

func (s *Store) GetGroup(ctx context.Context, id int64) (*domain.Group, error) {
	g, err := scanGroup(s.pool.QueryRow(ctx, groupSelect+` WHERE id=$1`, id))
	if err != nil {
		return nil, mapErr(err)
	}
	return g, nil
}

func (s *Store) CreateGroup(ctx context.Context, g *domain.Group) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO groups(name, description, channel_id, cache_strategy, transformer_config, rate_limit_config, retry_config, enabled)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, created_at, updated_at`,
		g.Name, g.Description, g.ChannelID, jsonbOf(g.CacheStrategy), jsonbOf(orEmptyArr(g.TransformerConfig)),
		jsonbOf(g.RateLimit), jsonbOf(g.Retry), g.Enabled,
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
}

func (s *Store) UpdateGroup(ctx context.Context, g *domain.Group) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE groups SET name=$2, description=$3, channel_id=$4, cache_strategy=$5, transformer_config=$6,
		 rate_limit_config=$7, retry_config=$8, enabled=$9, updated_at=NOW() WHERE id=$1`,
		g.ID, g.Name, g.Description, g.ChannelID, jsonbOf(g.CacheStrategy), jsonbOf(orEmptyArr(g.TransformerConfig)),
		jsonbOf(g.RateLimit), jsonbOf(g.Retry), g.Enabled)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM groups WHERE id=$1`, id)
	return err
}

// ---- API Keys ----

const apiKeySelect = `SELECT id, key_prefix, key_hash, COALESCE(key_encrypted,''), COALESCE(name,''), group_id, enabled, expires_at, created_at FROM api_keys`

func (s *Store) ListAPIKeys(ctx context.Context) ([]domain.APIKey, error) {
	rows, err := s.pool.Query(ctx, apiKeySelect+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (s *Store) GetAPIKey(ctx context.Context, id int64) (*domain.APIKey, error) {
	k, err := scanAPIKey(s.pool.QueryRow(ctx, apiKeySelect+` WHERE id=$1`, id))
	if err != nil {
		return nil, mapErr(err)
	}
	return k, nil
}

func (s *Store) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error) {
	k, err := scanAPIKey(s.pool.QueryRow(ctx, apiKeySelect+` WHERE key_prefix=$1`, prefix))
	if err != nil {
		return nil, mapErr(err)
	}
	return k, nil
}

func (s *Store) CreateAPIKey(ctx context.Context, k *domain.APIKey) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO api_keys(key_prefix, key_hash, key_encrypted, name, group_id, enabled, expires_at)
		 VALUES($1,$2,NULLIF($3,''),$4,$5,$6,$7) RETURNING id, created_at`,
		k.KeyPrefix, k.KeyHash, k.KeyEncrypted, k.Name, k.GroupID, k.Enabled, k.ExpiresAt,
	).Scan(&k.ID, &k.CreatedAt)
}

func (s *Store) UpdateAPIKey(ctx context.Context, k *domain.APIKey) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET name=$2, group_id=$3, enabled=$4, expires_at=$5 WHERE id=$1`,
		k.ID, k.Name, k.GroupID, k.Enabled, k.ExpiresAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAPIKey(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM api_keys WHERE id=$1`, id)
	return err
}

// ---- Model Mappings ----

func (s *Store) ListModelMappings(ctx context.Context, channelID int64) ([]domain.ModelMapping, error) {
	q := `SELECT id, channel_id, client_model, upstream_model FROM model_mappings`
	args := []any{}
	if channelID != 0 {
		q += ` WHERE channel_id=$1`
		args = append(args, channelID)
	}
	q += ` ORDER BY id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ModelMapping
	for rows.Next() {
		var m domain.ModelMapping
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.ClientModel, &m.UpstreamModel); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CreateModelMapping(ctx context.Context, m *domain.ModelMapping) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO model_mappings(channel_id, client_model, upstream_model) VALUES($1,$2,$3) RETURNING id`,
		m.ChannelID, m.ClientModel, m.UpstreamModel,
	).Scan(&m.ID)
}

func (s *Store) DeleteModelMapping(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM model_mappings WHERE id=$1`, id)
	return err
}

// ---- scan 辅助 ----

// rowScanner 兼容 pgx.Row 与 pgx.Rows。
type rowScanner interface{ Scan(dest ...any) error }

func scanChannel(r rowScanner) (*domain.UpstreamChannel, error) {
	var c domain.UpstreamChannel
	var typ string
	var cfg []byte
	if err := r.Scan(&c.ID, &c.Name, &typ, &c.BaseURL, &cfg, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	c.Type = domain.ChannelType(typ)
	c.Config = map[string]any{}
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &c.Config)
	}
	return &c, nil
}

const groupSelect = `SELECT id, name, COALESCE(description,''), channel_id, cache_strategy, transformer_config, rate_limit_config, retry_config, enabled, created_at, updated_at FROM groups`

func scanGroup(r rowScanner) (*domain.Group, error) {
	var g domain.Group
	var cs, tc, rl, rt []byte
	if err := r.Scan(&g.ID, &g.Name, &g.Description, &g.ChannelID, &cs, &tc, &rl, &rt, &g.Enabled, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(cs, &g.CacheStrategy)
	_ = json.Unmarshal(tc, &g.TransformerConfig)
	_ = json.Unmarshal(rl, &g.RateLimit)
	_ = json.Unmarshal(rt, &g.Retry)
	return &g, nil
}

func scanAPIKey(r rowScanner) (*domain.APIKey, error) {
	var k domain.APIKey
	if err := r.Scan(&k.ID, &k.KeyPrefix, &k.KeyHash, &k.KeyEncrypted, &k.Name, &k.GroupID, &k.Enabled, &k.ExpiresAt, &k.CreatedAt); err != nil {
		return nil, err
	}
	return &k, nil
}

func jsonbOf(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		return []byte("null")
	}
	return b
}

// orEmptyArr 保证 transformer_config 序列化为 [] 而非 null。
func orEmptyArr(v []domain.TransformerConfig) []domain.TransformerConfig {
	if v == nil {
		return []domain.TransformerConfig{}
	}
	return v
}
