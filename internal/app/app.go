// Package app 是装配根（composition root）：把存储、缓存、解析器、上游、
// 代理引擎、管理 API、静态前端组装为单一 http.Handler（任务书 §8 同进程）。
//
// 两种装配模式：
//   - BuildMemory：内存实现 + 种子数据，零外部依赖即可端到端运行与自测；
//   - BuildReal：连真实 PG/Redis/ClickHouse/S3，数据真正落库，readyz 真探活。
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/claude-gate/claude-gate/internal/admin"
	"github.com/claude-gate/claude-gate/internal/auth"
	"github.com/claude-gate/claude-gate/internal/config"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/gateway"
	"github.com/claude-gate/claude-gate/internal/observ"
	"github.com/claude-gate/claude-gate/internal/ratelimit"
	"github.com/claude-gate/claude-gate/internal/store"
	"github.com/claude-gate/claude-gate/internal/store/authstore"
	chstore "github.com/claude-gate/claude-gate/internal/store/clickhouse"
	"github.com/claude-gate/claude-gate/internal/store/memory"
	pgstore "github.com/claude-gate/claude-gate/internal/store/postgres"
	redisstore "github.com/claude-gate/claude-gate/internal/store/redis"
	s3store "github.com/claude-gate/claude-gate/internal/store/s3"
	"github.com/claude-gate/claude-gate/internal/upstream"
	"github.com/claude-gate/claude-gate/internal/upstream/keypool"
)

// App 持有装配后的运行时与对外 Handler。
type App struct {
	Handler http.Handler
	Config  config.Config
	Logger  *slog.Logger

	closers []func(context.Context) error
}

// Close 依次关闭所有资源（落库缓冲、连接池等）。
func (a *App) Close(ctx context.Context) error {
	var firstErr error
	for _, c := range a.closers {
		if err := c(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// components 是装配所需的存储与观测组件（内存或真实由调用方决定）。
type components struct {
	cfgStore      store.ConfigStore
	cache         store.Cache
	sink          observ.Sink
	bodies        observ.BodyStore
	metrics       observ.MetricsReader
	limiter       ratelimit.Limiter
	sampleSuccess float64
	ready         func() bool
	closers       []func(context.Context) error
}

// BuildMemory 用内存实现装配整套服务，并写入演示种子数据。
func BuildMemory(cfg config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	encKey := encKeyOf(cfg)
	cfgStore := memory.NewConfigStore()
	cfgStore.Seed(encKey)
	cache := memory.NewCache()

	obs := observ.NewMemoryStore()
	ctx := context.Background()
	channelByType := map[domain.ChannelType]int64{}
	if chs, err := cfgStore.ListChannels(ctx); err == nil {
		for _, ch := range chs {
			channelByType[ch.Type] = ch.ID
		}
	}
	var firstGroup int64
	if gs, err := cfgStore.ListGroups(ctx); err == nil && len(gs) > 0 {
		firstGroup = gs[0].ID
	}
	obs.SeedDemo(channelByType, firstGroup)
	limiter := ratelimit.NewMemory()

	return assemble(cfg, logger, components{
		cfgStore: cfgStore, cache: cache, sink: obs, bodies: obs, metrics: obs, limiter: limiter,
		sampleSuccess: 1.0, // 演示模式全量留存 body
		ready: func() bool { return true },
		// sink(obs).Close 由 assemble 统一追加；此处仅限流器
		closers: []func(context.Context) error{
			func(context.Context) error { limiter.Close(); return nil },
		},
	})
}

// BuildReal 连接真实存储装配整套服务。数据真正落库，readyz 探活全部依赖。
func BuildReal(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}

	pg, err := pgstore.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		return nil, err
	}
	rds, err := redisstore.New(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		pg.Close()
		return nil, err
	}
	chAddr, chDB, chUser, chPass := parseCHDSN(cfg.ClickHouse.DSN)
	ch, err := chstore.New(ctx, chstore.Options{Addr: chAddr, Database: chDB, Username: chUser, Password: chPass, Logger: logger})
	if err != nil {
		pg.Close()
		_ = rds.Close()
		return nil, err
	}
	s3s, err := s3store.New(ctx, s3store.Options{
		Endpoint: cfg.S3.Endpoint, AccessKey: cfg.S3.AccessKey, SecretKey: cfg.S3.SecretKey,
		Bucket: cfg.S3.Bucket, UseSSL: cfg.S3.UseSSL,
	})
	if err != nil {
		pg.Close()
		_ = rds.Close()
		_ = ch.Close(ctx)
		return nil, err
	}

	ensureAdmin(ctx, pg, logger)
	if chs, _ := pg.ListChannels(ctx); len(chs) == 0 {
		memory.SeedConfigStore(ctx, pg, encKeyOf(cfg))
		logger.Info("真实库首次启动，已写入演示种子数据（含自测 Key）")
	}

	ready := func() bool {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pg.Ping(c) == nil && rds.Ping(c) == nil && ch.Ping(c) == nil && s3s.Ping(c) == nil
	}
	sample := cfg.Sampling.SuccessRate
	if sample <= 0 {
		sample = 0.01
	}
	logger.Info("已连接真实存储", "pg", true, "redis", cfg.Redis.Addr, "clickhouse", chAddr, "s3", cfg.S3.Endpoint)
	return assemble(cfg, logger, components{
		cfgStore: pg, cache: rds, sink: ch, bodies: s3s, metrics: ch,
		limiter:       ratelimit.NewRedis(rds.Client()), // 分布式限流，复用 Redis 连接池
		sampleSuccess: sample, ready: ready,
		// sink(ch).Close 由 assemble 统一追加（flush CH 批写缓冲）；此处关连接池
		closers: []func(context.Context) error{
			func(context.Context) error { pg.Close(); return nil },
			func(context.Context) error { return rds.Close() },
		},
	})
}

// assemble 是公共装配逻辑：网关 + 管理 API + 静态前端。
func assemble(cfg config.Config, logger *slog.Logger, c components) (*App, error) {
	ctx := context.Background()
	encKey := encKeyOf(cfg)

	authAdapter := authstore.New(c.cfgStore, c.cache, cfg.Auth.APIKeyTTLSec)
	resolver := auth.NewResolver(authAdapter)

	registry := upstream.DefaultRegistry()
	pool := keypool.New(0)
	loadPoolKeys(ctx, c.cfgStore, pool)
	// 运行时通过管理 API 新增/启停 Key 后，重载该通道的选择池使其立即生效
	reloadKeys := func(rctx context.Context, channelID int64) {
		keys, err := c.cfgStore.ListUpstreamKeys(rctx, channelID)
		if err != nil {
			return
		}
		ptrs := make([]*domain.UpstreamKey, 0, len(keys))
		for i := range keys {
			k := keys[i]
			ptrs = append(ptrs, &k)
		}
		pool.Load(channelID, ptrs)
	}

	proxy := gateway.NewProxy(gateway.ProxyDeps{
		Resolver:      resolver,
		Registry:      registry,
		Pool:          pool,
		ConfigStore:   c.cfgStore,
		Sink:          c.sink,
		Bodies:        c.bodies,
		Logger:        logger,
		SampleSuccess: c.sampleSuccess,
		Timeout:       time.Duration(cfg.Server.RequestTimeout) * time.Second,
		EncKey:        encKey,

		GlobalMaxInFlight:  cfg.Concurrency.GlobalMaxInFlight,
		PerChannelInFlight: cfg.Concurrency.PerChannelInFlight,
		Limiter:            c.limiter,
		WorkerPoolSize:     cfg.Concurrency.WorkerPoolSize,
		S3WriteRetry:       cfg.Sampling.S3WriteRetry,
	})

	mux := http.NewServeMux()
	gw := gateway.NewServer(logger, c.ready)
	gw.SetProxy(proxy)
	gw.Mount(mux)

	adminSrv := admin.NewServer(admin.Deps{
		Store: c.cfgStore, Metrics: c.metrics, Bodies: c.bodies, Replayer: proxy, Invalidator: authAdapter,
		KeyReloader: reloadKeys,
		JWTSecret:   orDefault(cfg.Auth.JWTSecret, "claude-gate-dev-jwt-secret"),
		JWTTTL:    time.Duration(maxInt(cfg.Auth.JWTTTLMinutes, 1440)) * time.Minute,
		EncKey:    encKey, Logger: logger,
	})
	adminSrv.Mount(mux)
	mountStatic(mux, logger)

	// 关闭顺序：先 flush 代理落库队列（body→S3），再 flush 明细 Sink（→CH），最后关连接池
	closers := []func(context.Context) error{proxy.Close}
	if c.sink != nil {
		closers = append(closers, c.sink.Close)
	}
	closers = append(closers, c.closers...)
	return &App{Handler: mux, Config: cfg, Logger: logger, closers: closers}, nil
}

// ensureAdmin 在真实库首次启动且无用户时，创建默认管理员，保证可登录。
func ensureAdmin(ctx context.Context, cfgStore store.ConfigStore, logger *slog.Logger) {
	const email = "admin@claude-gate.io"
	if _, err := cfgStore.GetUserByEmail(ctx, email); err == nil {
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		logger.Warn("检查默认管理员失败", "err", err)
		return
	}
	if err := cfgStore.CreateUser(ctx, &domain.User{Email: email, PasswordHash: auth.HashSecret("admin123"), Role: "admin"}); err != nil {
		logger.Warn("创建默认管理员失败", "err", err)
		return
	}
	logger.Info("已创建默认管理员", "email", email, "password", "admin123（请尽快修改）")
}

func loadPoolKeys(ctx context.Context, cfgStore store.ConfigStore, pool *keypool.MemoryPool) {
	chs, err := cfgStore.ListChannels(ctx)
	if err != nil {
		return
	}
	for _, ch := range chs {
		keys, err := cfgStore.ListUpstreamKeys(ctx, ch.ID)
		if err != nil {
			continue
		}
		ptrs := make([]*domain.UpstreamKey, 0, len(keys))
		for i := range keys {
			k := keys[i]
			ptrs = append(ptrs, &k)
		}
		pool.Load(ch.ID, ptrs)
	}
}

func mountStatic(mux *http.ServeMux, logger *slog.Logger) {
	dir := firstExisting("web/dist", "../web/dist", "/app/web")
	if dir == "" {
		logger.Info("未找到前端构建产物，跳过静态托管（可单独运行 web dev server）")
		return
	}
	fs := http.FileServer(http.Dir(dir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
	logger.Info("已托管前端静态产物", "dir", dir)
}

// parseCHDSN 从 clickhouse://user:pass@host:9000/db 解析出连接参数。
func parseCHDSN(dsn string) (addr, db, user, pass string) {
	addr, db = "localhost:9000", "claude_gate"
	u, err := url.Parse(dsn)
	if err != nil {
		return
	}
	if u.Host != "" {
		addr = u.Host
	}
	if p := strings.TrimPrefix(u.Path, "/"); p != "" {
		db = p
	}
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	return
}

func encKeyOf(cfg config.Config) string {
	if cfg.Auth.EncryptionKey != "" {
		return cfg.Auth.EncryptionKey
	}
	return "claude-gate-dev-encryption-key"
}

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
