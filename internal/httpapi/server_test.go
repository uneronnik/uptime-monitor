package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type observedListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

func (l *observedListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { close(l.closed) })
	return err
}

func TestHealth(t *testing.T) {
	for _, tt := range []struct {
		method, path, body string
		code               int
		want               string
	}{
		{"GET", "/healthz", "", 200, "{\"status\":\"ok\"}\n"},
		{"HEAD", "/healthz", "", 200, ""},
		{"POST", "/healthz", "", 405, ""},
		{"GET", "/healthz", "unexpected", 413, ""},
		{"GET", "/healthz/extra", "", 404, ""},
		{"GET", "/readyz", "", 503, ""},
		{"GET", "/api/monitors", "", 404, ""},
	} {
		t.Run(tt.method+tt.path+tt.body, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Handler(nil).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d", rec.Code, tt.code)
			}
			if tt.code == 200 && (rec.Body.String() != tt.want || rec.Header().Get("Content-Type") != "application/json") {
				t.Fatalf("unexpected response: %v %q", rec.Header(), rec.Body.String())
			}
		})
	}
}

func TestServeShutdown(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "drain", true: "deadline"}[force], func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			observed := &observedListener{Listener: listener, closed: make(chan struct{})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			release := make(chan struct{})
			canceled := make(chan struct{})
			var releaseOnce sync.Once
			finish := func() { releaseOnce.Do(func() { close(release) }) }
			defer finish()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(started)
				select {
				case <-release:
					io.WriteString(w, "finished")
				case <-r.Context().Done():
					close(canceled)
				}
			})
			done := make(chan error, 1)
			timeout := 2 * time.Second
			if force {
				timeout = 30 * time.Millisecond
			}
			go func() { done <- Serve(ctx, observed, timeout, handler, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
			response := make(chan string, 1)
			client := http.Client{Timeout: 3 * time.Second}
			go func() {
				res, err := client.Get("http://" + listener.Addr().String())
				if err != nil {
					response <- "request failed"
					return
				}
				defer res.Body.Close()
				b, _ := io.ReadAll(res.Body)
				response <- string(b)
			}()
			select {
			case <-started:
			case err := <-done:
				t.Fatalf("server exited before request: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			select {
			case <-observed.closed:
			case <-time.After(time.Second):
				t.Fatal("listener did not close")
			}
			if !force {
				select {
				case err := <-done:
					t.Fatalf("shutdown did not drain: %v", err)
				case <-canceled:
					t.Fatal("request canceled before deadline")
				default:
				}
				finish()
			}
			select {
			case err := <-done:
				if (err != nil) != force {
					t.Fatalf("shutdown error = %v, forced = %v", err, force)
				}
				if force && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected deadline error, got %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("shutdown exceeded bound")
			}
			if force {
				select {
				case <-canceled:
				case <-time.After(time.Second):
					t.Fatal("active request not canceled")
				}
			} else {
				select {
				case got := <-response:
					if got != "finished" {
						t.Fatalf("response = %q", got)
					}
				case <-time.After(time.Second):
					t.Fatal("no response")
				}
			}
		})
	}
}

func TestHealthRejectsUnknownLengthBody(t *testing.T) {
	request := httptest.NewRequest("GET", "/healthz", strings.NewReader("body"))
	request.ContentLength = -1
	request.TransferEncoding = []string{"chunked"}
	rec := httptest.NewRecorder()
	Handler(nil).ServeHTTP(rec, request)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestServeHeaderLimit(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, listener, time.Second, Handler(nil), slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	client := http.Client{Timeout: 3 * time.Second}
	request, err := http.NewRequest("GET", "http://"+listener.Addr().String()+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Large", strings.Repeat("x", 32<<10))
	res, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status = %d", res.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}
