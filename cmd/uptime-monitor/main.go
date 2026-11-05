package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uneronnik/uptime-monitor/internal/config"
	"github.com/uneronnik/uptime-monitor/internal/httpapi"
	"github.com/uneronnik/uptime-monitor/internal/postgres"
)

func main() { os.Exit(run()) }

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		logger.Error("invalid_configuration", "reason", err.Error())
		return 1
	}
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	databaseURL, err := config.DatabaseURL(os.LookupEnv)
	if err != nil {
		logger.Error("invalid_configuration", "reason", err.Error())
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 5*time.Second)
	pool, err := postgres.Open(startup, databaseURL)
	cancel()
	if err != nil {
		logger.Error("database_unavailable")
		return 1
	}
	defer pool.Close()
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		logger.Error("listen_failed")
		return 1
	}
	logger.Info("http_started")
	if err := httpapi.Serve(ctx, listener, cfg.ShutdownTimeout, httpapi.Handler(pool.Ping), logger); err != nil {
		logger.Error("server_failed")
		return 1
	}
	logger.Info("shutdown_completed")
	return 0
}
