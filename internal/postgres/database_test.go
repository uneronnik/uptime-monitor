package postgres

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3/lock"
)

func testDatabase(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("set TEST_DATABASE_URL to a separate test database")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("test database connection failed")
	}
	defer conn.Close(context.Background())
	schema := fmt.Sprintf("test_%x", rand.Text())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal("test schema creation failed")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Error("cleanup connection failed")
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error("test schema cleanup failed")
		}
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

func TestMigrationsPreserveDataAndConstraints(t *testing.T) {
	dsn := testDatabase(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	err = pool.QueryRow(ctx, "INSERT INTO monitors (name, url) VALUES ('Example', 'https://example.com') RETURNING id").Scan(&id)
	if err != nil {
		pool.Close()
		t.Fatal("initial monitor insert failed")
	}
	pool.Close()
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err = Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var name string
	var status, interval, timeout int
	var paused bool
	err = pool.QueryRow(ctx, "SELECT name, expected_status, interval_seconds, timeout_ms, paused FROM monitors WHERE id=$1", id).Scan(&name, &status, &interval, &timeout, &paused)
	if err != nil || name != "Example" || status != 200 || interval != 60 || timeout != 5000 || paused {
		t.Fatalf("persisted defaults differ: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id=1 AND is_applied").Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration recorded %d times: %v", count, err)
	}
	for _, fields := range []string{
		"'', 'https://example.com', 200, 60, 5000",
		"'Example', '', 200, 60, 5000",
		"'Example', 'https://example.com', 99, 60, 5000",
		"'Example', 'https://example.com', 600, 60, 5000",
		"'Example', 'https://example.com', 200, 29, 5000",
		"'Example', 'https://example.com', 200, 30, 0",
		"'Example', 'https://example.com', 200, 30, 30000",
	} {
		_, err := pool.Exec(ctx, "INSERT INTO monitors (name,url,expected_status,interval_seconds,timeout_ms) VALUES ("+fields+")")
		if err == nil {
			t.Fatalf("invalid row accepted: %s", fields)
		}
	}
	if _, err := pool.Exec(ctx, "INSERT INTO monitors (name,url,interval_seconds,timeout_ms) VALUES ('Boundary','https://example.com',30,29999)"); err != nil {
		t.Fatal("valid timeout boundary rejected")
	}
}

func TestOpenHonorsCancellationAndSanitizesErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	pool, err := Open(ctx, "postgres://owner:secret@127.0.0.1:1/uptime?sslmode=disable")
	if pool != nil {
		pool.Close()
		t.Fatal("canceled open returned a pool")
	}
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("missing or unsafe connection error")
	}
}

func TestOpenFailureIsBounded(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	pool, err := Open(ctx, "postgres://owner:secret@"+listener.Addr().String()+"/uptime?sslmode=disable")
	if pool != nil {
		pool.Close()
		t.Fatal("unresponsive database accepted")
	}
	if err == nil || ctx.Err() == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("deadline ignored or unsafe error")
	}
}

func TestMigrationFailureRollsBackAndCanRetry(t *testing.T) {
	dsn := testDatabase(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	files := fstest.MapFS{"00001_broken.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE partial_change (id int);\nSELECT secret_missing_function();\n-- +goose Down\nDROP TABLE partial_change;\n")}}
	err := migrate(ctx, dsn, files)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("missing or unsafe migration failure")
	}
	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var absent bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('partial_change') IS NULL").Scan(&absent); err != nil || !absent {
		t.Fatalf("partial DDL survived: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id=1 AND is_applied").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed migration marked applied: %v", err)
	}
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
}

func TestConcurrentMigrations(t *testing.T) {
	dsn := testDatabase(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() { <-start; results <- Migrate(ctx, dsn) }()
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id=1 AND is_applied").Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate or missing migration: %d, %v", count, err)
	}
}

func TestMigrationLockHonorsCancellation(t *testing.T) {
	dsn := testDatabase(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("lock holder connection failed")
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lock.DefaultLockID); err != nil {
		t.Fatal("could not hold migration lock")
	}
	limited, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	err = Migrate(limited, dsn)
	stop()
	if err == nil {
		t.Fatal("migration ran while another session held its lock")
	}
	var absent bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass('monitors') IS NULL").Scan(&absent); err != nil || !absent {
		t.Fatalf("canceled migration changed schema: %v", err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lock.DefaultLockID); err != nil {
		t.Fatal("could not release migration lock")
	}
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
}
