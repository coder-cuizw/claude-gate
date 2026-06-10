// Package app 是装配根（composition root）：把存储、缓存、解析器、上游、
// 代理引擎、管理 API、静态前端组装为单一 http.Handler（任务书 §8 同进程）。
//
// 当前提供内存模式（memory）：零外部依赖即可端到端运行与自测；真实存储
// （PG/CH/Redis/S3）实现同样接口后可在此切换接入。
package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/claude-gate/claude-gate/internal/admin"
	"github.com/claude-gate/claude-gate/internal/auth"
	"github.com/claude-gate/claude-gate/internal/config"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/gateway"
	"github.com/claude-gate/claude-gate/internal/observ"
	"github.com/claude-gate/claude-gate/internal/store"
	"github.com/claude-gate/claude-gate/internal/store/authstore"
	"github.com/claude-gate/claude-gate/internal/store/memory"
	"github.com/claude-gate/claude-gate/internal/upstream"
	"github.com/claude-gate/claude-gate/internal/upstream/keypool"
)

// App 持有装配后的运行时与对外 Handler。
type App struct {
	Handler http.Handler
	Config  config.Config
	Logger  *slog.Logger

	sink observ.Sink
}

// BuildMemory 用内存实现装配整套服务，并写入演示种子数据。
func BuildMemory(cfg config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	encKey := cfg.Auth.EncryptionKey
	if encKey == "" {
		encKey = "claude-gate-dev-encryption-key"
	}

	// 配置存储 + 种子
	cfgStore := memory.NewConfigStore()
	cfgStore.Seed(encKey)
	cache := memory.NewCache()

	// 观测存储（Sink + BodyStore + MetricsReader）+ 演示历史明细
	obs := observ.NewMemoryStore()
	ctx := context.Background()
	channelByType := map[domain.ChannelType]int64{}
	var passthroughGroupID int64
	if chs, err := cfgStore.ListChannels(ctx); err == nil {
		for _, ch := range chs {
			channelByType[ch.Type] = ch.ID
		}
	}
	if gs, err := cfgStore.ListGroups(ctx); err == nil && len(gs) > 0 {
		passthroughGroupID = gs[0].ID
	}
	obs.SeedDemo(channelByType, passthroughGroupID)

	// 解析器（带缓存的 auth.Store 适配）
	authAdapter := authstore.New(cfgStore, cache, cfg.Auth.APIKeyTTLSec)
	resolver := auth.NewResolver(authAdapter)

	// 上游注册表 + Key 选择器（加载各通道 Key）
	registry := upstream.DefaultRegistry()
	pool := keypool.New(0)
	loadPoolKeys(ctx, cfgStore, pool)

	// 代理引擎
	proxy := gateway.NewProxy(gateway.ProxyDeps{
		Resolver:      resolver,
		Registry:      registry,
		Pool:          pool,
		ConfigStore:   cfgStore,
		Sink:   obs,
		Bodies: obs,
		Logger: logger,
		// 内存演示模式下全量留存 body，便于明细详情与请求复现开箱即用；
		// 生产真实模式应回落到 cfg.Sampling.SuccessRate（默认 1% 采样）。
		SampleSuccess: 1.0,
		Timeout:       time.Duration(cfg.Server.RequestTimeout) * time.Second,
	})

	// 组装单一 mux：网关 + 管理 API + 静态前端
	mux := http.NewServeMux()
	gw := gateway.NewServer(logger, func() bool { return true })
	gw.SetProxy(proxy)
	gw.Mount(mux)

	adminSrv := admin.NewServer(admin.Deps{
		Store:       cfgStore,
		Metrics:     obs,
		Bodies:      obs,
		Replayer:    proxy,
		Invalidator: authAdapter,
		JWTSecret:   orDefault(cfg.Auth.JWTSecret, "claude-gate-dev-jwt-secret"),
		JWTTTL:      time.Duration(maxInt(cfg.Auth.JWTTTLMinutes, 1440)) * time.Minute,
		EncKey:      encKey,
		Logger:      logger,
	})
	adminSrv.Mount(mux)

	mountStatic(mux, logger)

	logger.Info("装配完成（内存模式）", "channels", len(channelByType), "self_test_api_key", memory.SelfTestAPIKey)
	return &App{Handler: mux, Config: cfg, Logger: logger, sink: obs}, nil
}

// Close 优雅关闭：flush 落库缓冲。
func (a *App) Close(ctx context.Context) error {
	if a.sink != nil {
		return a.sink.Close(ctx)
	}
	return nil
}

// loadPoolKeys 把各通道的 active Key 加载进选择器。
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

// mountStatic 在存在前端构建产物时托管 SPA（带 history fallback）。
func mountStatic(mux *http.ServeMux, logger *slog.Logger) {
	dir := firstExisting("web/dist", "../web/dist", "/app/web")
	if dir == "" {
		logger.Info("未找到前端构建产物，跳过静态托管（可单独运行 web dev server）")
		return
	}
	fs := http.FileServer(http.Dir(dir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 静态文件存在则直接返回，否则回退 index.html（前端路由）
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
	logger.Info("已托管前端静态产物", "dir", dir)
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
