package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{name: "defaults", want: Config{"127.0.0.1:8080", 10 * time.Second, slog.LevelInfo}},
		{name: "custom", env: map[string]string{"HTTP_ADDR": "[::1]:9090", "SHUTDOWN_TIMEOUT": "250ms", "LOG_LEVEL": "debug"}, want: Config{"[::1]:9090", 250 * time.Millisecond, slog.LevelDebug}},
		{name: "all interfaces", env: map[string]string{"HTTP_ADDR": ":8080"}, want: Config{":8080", 10 * time.Second, slog.LevelInfo}},
		{name: "upper limits", env: map[string]string{"HTTP_ADDR": "127.0.0.1:65535", "SHUTDOWN_TIMEOUT": "1m", "LOG_LEVEL": "WARN"}, want: Config{"127.0.0.1:65535", time.Minute, slog.LevelWarn}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(func(k string) (string, bool) { v, ok := tt.env[k]; return v, ok })
			if err != nil || got != tt.want {
				t.Fatalf("Load() = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestLoadRejectsInvalidValuesWithoutEchoingThem(t *testing.T) {
	for key, values := range map[string][]string{
		"HTTP_ADDR":        {"", "secret.example:8080", "127.0.0.1:0", ":65536", ":+8080", "127.0.0.1", "user:secret@host:80"},
		"SHUTDOWN_TIMEOUT": {"", "secret", "0s", "-1s", "61s"},
		"LOG_LEVEL":        {"", "secret", "TRACE"},
	} {
		for _, value := range values {
			t.Run(key+"/"+value, func(t *testing.T) {
				_, err := Load(func(k string) (string, bool) { return value, k == key })
				if err == nil {
					t.Fatal("invalid configuration accepted")
				}
				if !strings.Contains(err.Error(), key) {
					t.Fatalf("missing setting name: %v", err)
				}
				if strings.Contains(value, "secret") && strings.Contains(err.Error(), "secret") {
					t.Fatalf("value leaked: %v", err)
				}
			})
		}
	}
}
