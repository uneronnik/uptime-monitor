package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uneronnik/uptime-monitor/internal/config"
	"github.com/uneronnik/uptime-monitor/internal/postgres"
)

func main() { os.Exit(run()) }

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) != 1 {
		logger.Error("unexpected_arguments")
		return 1
	}
	databaseURL, err := config.DatabaseURL(os.LookupEnv)
	if err != nil {
		logger.Error("invalid_configuration", "reason", err.Error())
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := postgres.Migrate(ctx, databaseURL); err != nil {
		logger.Error("migration_failed")
		return 1
	}
	logger.Info("migrations_applied")
	return 0
}
