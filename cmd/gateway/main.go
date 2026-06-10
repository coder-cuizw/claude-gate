// Command gateway 是 claude-gate 的主进程：网关入口 + 管理 API（同进程，任务书 §8）。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/claude-gate/claude-gate/internal/config"
	"github.com/claude-gate/claude-gate/internal/gateway"
)

func main() {
	cfgPath := flag.String("config", os.Getenv("CG_CONFIG"), "配置文件路径（YAML），可空")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("加载配置失败", "err", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.Log)
	slog.SetDefault(logger)

	srv := gateway.NewServer(logger, func() bool { return true })
	httpServer := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: srv.Handler(),
	}

	// 启动 HTTP 服务
	go func() {
		logger.Info("claude-gate 网关启动", "addr", cfg.Server.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP 服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	// 等待退出信号，优雅关闭
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("收到退出信号，开始优雅关闭")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("优雅关闭超时", "err", err)
	}
	logger.Info("claude-gate 已退出")
}

// newLogger 按配置构造结构化日志器（任务书 §10：slog 结构化）。
func newLogger(cfg config.LogConfig) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler = slog.NewJSONHandler(os.Stdout, opts)
	if cfg.Format == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}
