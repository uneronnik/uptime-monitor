package httpapi

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"time"
)

func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength != 0 {
			http.Error(w, "request body not allowed", http.StatusRequestEntityTooLarge)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodHead {
			io.WriteString(w, "{\"status\":\"ok\"}\n")
		}
	})
	return mux
}

// transportLog deliberately drops raw transport diagnostics, which can contain input.
type transportLog struct{ logger *slog.Logger }

func (w transportLog) Write(p []byte) (int, error) {
	w.logger.Warn("http_transport_error")
	return len(p), nil
}

func Serve(ctx context.Context, listener net.Listener, timeout time.Duration, handler http.Handler, logger *slog.Logger) error {
	requests, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		BaseContext:       func(net.Listener) context.Context { return requests },
		ErrorLog:          log.New(transportLog{logger}, "", 0),
	}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	select {
	case err := <-result:
		_ = server.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	logger.Info("shutdown_started")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := server.Shutdown(shutdownCtx)
	if err != nil {
		cancelRequests()
		_ = server.Close()
	}
	serveErr := <-result
	if err != nil {
		return err
	}
	if !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}
