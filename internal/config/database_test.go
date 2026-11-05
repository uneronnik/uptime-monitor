package config

import (
	"strings"
	"testing"
)

func TestDatabaseURL(t *testing.T) {
	for _, value := range []string{"", "secret", "https://secret/db", "postgres://", "postgres://localhost", "postgres://user:secret@localhost/db?sslmode=invalid"} {
		t.Run(value, func(t *testing.T) {
			_, err := DatabaseURL(func(string) (string, bool) { return value, value != "" })
			if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
	want := "postgres://owner:password@127.0.0.1:5432/uptime?sslmode=disable"
	got, err := DatabaseURL(func(string) (string, bool) { return want, true })
	if err != nil || got != want {
		t.Fatalf("valid database URL rejected: %v", err)
	}
}
