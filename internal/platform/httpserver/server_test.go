package httpserver_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"sre-kit/internal/platform/httpserver"
)

func TestUse_MiddlewareWrapsMux(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-listener test in short mode")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := httpserver.New(addr)
	srv.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	var sawMiddleware bool
	srv.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sawMiddleware = true
			next.ServeHTTP(w, r)
		})
	})

	done := make(chan error, 1)
	go func() { done <- srv.Start() }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	waitForServer(t, addr)

	resp, err := http.Get("http://" + addr + "/ok")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if !sawMiddleware {
		t.Fatal("registered middleware was never invoked")
	}
}

func waitForServer(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s never started listening", addr)
}
