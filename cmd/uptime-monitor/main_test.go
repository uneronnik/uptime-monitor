package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProcessHelper(t *testing.T) {
	if os.Getenv("UPTIME_TEST_PROCESS") == "1" {
		os.Exit(run())
	}
}

func process(t *testing.T, settings map[string]string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessHelper$")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, ok := settings[key]; !ok && key != "UPTIME_TEST_PROCESS" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "UPTIME_TEST_PROCESS=1")
	for key, value := range settings {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	return cmd
}

func TestProcessInvalidConfiguration(t *testing.T) {
	cmd := process(t, map[string]string{"HTTP_ADDR": "secret-value", "LOG_LEVEL": "INFO", "SHUTDOWN_TIMEOUT": "1s"})
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("invalid configuration exited successfully")
	}
	var event map[string]any
	if err := json.Unmarshal(output, &event); err != nil {
		t.Fatalf("invalid JSON log: %q", output)
	}
	if event["msg"] != "invalid_configuration" || strings.Contains(string(output), "secret-value") {
		t.Fatalf("unsafe or missing error: %s", output)
	}
}

func TestProcessBindFailure(t *testing.T) {
	requireDatabase(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cmd := process(t, map[string]string{"HTTP_ADDR": listener.Addr().String(), "LOG_LEVEL": "INFO", "SHUTDOWN_TIMEOUT": "1s"})
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("bind failure exited successfully")
	}
	var event map[string]any
	if err := json.Unmarshal(output, &event); err != nil {
		t.Fatalf("invalid JSON log: %q", output)
	}
	if event["msg"] != "listen_failed" || strings.Contains(string(output), listener.Addr().String()) {
		t.Fatalf("unsafe or missing error: %s", output)
	}
}

func TestProcessLifecycle(t *testing.T) {
	requireDatabase(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	cmd := process(t, map[string]string{"HTTP_ADDR": address, "LOG_LEVEL": "INFO", "SHUTDOWN_TIMEOUT": "1s"})
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(line, "http_started") {
		t.Fatalf("startup: %q, %v", line, err)
	}
	client := http.Client{Timeout: 2 * time.Second}
	res, err := client.Get("http://" + address + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil || res.StatusCode != 200 || string(body) != "{\"status\":\"ok\"}\n" {
		t.Fatalf("health: %d %q %v", res.StatusCode, body, err)
	}
	res, err = client.Get("http://" + address + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil || res.StatusCode != 200 || string(body) != "{\"status\":\"ready\"}\n" {
		t.Fatalf("readiness: %d %q %v", res.StatusCode, body, err)
	}
	if runtime.GOOS == "windows" {
		return
	} // Windows process.Signal cannot deliver console Ctrl+C.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	remaining, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(remaining), "shutdown_completed") {
		t.Fatalf("shutdown: %s", remaining)
	}
}

func requireDatabase(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("TEST_DATABASE_URL required in CI")
		}
		t.Skip("set TEST_DATABASE_URL to a separate test database")
	}
	t.Setenv("DATABASE_URL", dsn)
}

func TestProcessDatabaseFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	cmd := process(t, map[string]string{"DATABASE_URL": "postgres://owner:secret@" + address + "/db?sslmode=disable", "HTTP_ADDR": "127.0.0.1:8080", "SHUTDOWN_TIMEOUT": "1s", "LOG_LEVEL": "INFO"})
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "database_unavailable") || strings.Contains(string(output), "secret") || strings.Contains(string(output), "http_started") {
		t.Fatalf("missing or unsafe database startup failure: %q, %v", output, err)
	}
}

func TestProcessRequiresDatabaseConfiguration(t *testing.T) {
	cmd := process(t, map[string]string{"DATABASE_URL": "", "HTTP_ADDR": "127.0.0.1:8080", "SHUTDOWN_TIMEOUT": "1s", "LOG_LEVEL": "INFO"})
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "invalid_configuration") || !strings.Contains(string(output), "DATABASE_URL") {
		t.Fatalf("missing database configuration accepted: %q, %v", output, err)
	}
}
