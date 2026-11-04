package config

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration
	LogLevel        slog.Level
}

func Load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, fallback string) string {
		if value, ok := lookup(key); ok {
			return value
		}
		return fallback
	}
	cfg := Config{HTTPAddr: get("HTTP_ADDR", "127.0.0.1:8080")}
	host, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil {
		return Config{}, errors.New("HTTP_ADDR must be an IP address and port")
	}
	if host != "" {
		if _, err := netip.ParseAddr(host); err != nil {
			return Config{}, errors.New("HTTP_ADDR host must be a literal IP address")
		}
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 || strings.Trim(port, "0123456789") != "" {
		return Config{}, errors.New("HTTP_ADDR port must be between 1 and 65535")
	}
	cfg.ShutdownTimeout, err = time.ParseDuration(get("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || cfg.ShutdownTimeout <= 0 || cfg.ShutdownTimeout > time.Minute {
		return Config{}, errors.New("SHUTDOWN_TIMEOUT must be positive and at most 1m")
	}
	switch strings.ToUpper(get("LOG_LEVEL", "INFO")) {
	case "DEBUG":
		cfg.LogLevel = slog.LevelDebug
	case "INFO":
		cfg.LogLevel = slog.LevelInfo
	case "WARN":
		cfg.LogLevel = slog.LevelWarn
	case "ERROR":
		cfg.LogLevel = slog.LevelError
	default:
		return Config{}, errors.New("LOG_LEVEL must be DEBUG, INFO, WARN, or ERROR")
	}
	return cfg, nil
}
