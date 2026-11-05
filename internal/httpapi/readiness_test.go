package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadiness(t *testing.T) {
	for _, healthy := range []bool{true, false} {
		for _, method := range []string{"GET", "HEAD"} {
			called := false
			var child context.Context
			handler := Handler(func(ctx context.Context) error {
				called = true
				child = ctx
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 2*time.Second {
					t.Error("readiness query has no bounded deadline")
				}
				if !healthy {
					return errors.New("postgres://secret/password")
				}
				return nil
			})
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(method, "/readyz", nil))
			want := 200
			if !healthy {
				want = 503
			}
			if rec.Code != want || !called || strings.Contains(rec.Body.String(), "secret") {
				t.Fatalf("readiness = %d %q, called=%v", rec.Code, rec.Body.String(), called)
			}
			if method == "HEAD" && rec.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
			if method == "GET" && healthy && rec.Body.String() != "{\"status\":\"ready\"}\n" {
				t.Fatal("wrong success body")
			}
			if method == "GET" && !healthy && rec.Body.String() != "{\"status\":\"unavailable\"}\n" {
				t.Fatal("wrong failure body")
			}
			if child.Err() != context.Canceled {
				t.Fatal("readiness context was not released")
			}
		}
	}
}

func TestReadinessRequestLimitsAndHealthIndependence(t *testing.T) {
	handler := Handler(func(context.Context) error { t.Fatal("unexpected database query"); return nil })
	for _, tt := range []struct {
		method, path, body string
		code               int
	}{
		{"POST", "/readyz", "", 405},
		{"GET", "/readyz", "payload", 413},
		{"GET", "/healthz", "", 200},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if rec.Code != tt.code {
			t.Fatalf("%s %s = %d", tt.method, tt.path, rec.Code)
		}
	}
}

func TestReadinessRejectsUnknownLengthBody(t *testing.T) {
	request := httptest.NewRequest("GET", "/readyz", strings.NewReader("payload"))
	request.ContentLength = -1
	request.TransferEncoding = []string{"chunked"}
	rec := httptest.NewRecorder()
	Handler(func(context.Context) error { t.Fatal("body reached database check"); return nil }).ServeHTTP(rec, request)
	if rec.Code != 413 {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestReadinessPropagatesRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	handler := Handler(func(child context.Context) error {
		if !errors.Is(child.Err(), context.Canceled) {
			t.Error("lost request cancellation")
		}
		return child.Err()
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx))
	if rec.Code != 503 {
		t.Fatalf("status = %d", rec.Code)
	}
}
