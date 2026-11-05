package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMigrateProcessHelper(t *testing.T) {
	if os.Getenv("UPTIME_MIGRATE_HELPER") != "1" {
		return
	}
	os.Args = []string{os.Args[0]}
	if arg := os.Getenv("UPTIME_MIGRATE_ARGUMENT"); arg != "" {
		os.Args = append(os.Args, arg)
	}
	os.Exit(run())
}

func TestMigrateProcessFailures(t *testing.T) {
	for _, tt := range []struct{ name, dsn, argument, event string }{
		{"missing configuration", "", "", "invalid_configuration"},
		{"invalid configuration", "postgres://secret", "", "invalid_configuration"},
		{"unavailable database", "postgres://owner:secret@127.0.0.1:1/db?sslmode=disable", "", "migration_failed"},
		{"destructive argument", "", "down", "unexpected_arguments"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMigrateProcessHelper$")
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				if key != "DATABASE_URL" && key != "UPTIME_MIGRATE_HELPER" && key != "UPTIME_MIGRATE_ARGUMENT" {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "UPTIME_MIGRATE_HELPER=1", "DATABASE_URL="+tt.dsn, "UPTIME_MIGRATE_ARGUMENT="+tt.argument)
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tt.event) || strings.Contains(string(output), "secret") {
				t.Fatalf("missing or unsafe failure: %q, %v", output, err)
			}
		})
	}
}
