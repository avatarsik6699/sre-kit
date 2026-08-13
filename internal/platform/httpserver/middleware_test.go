package httpserver_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"sre-kit/internal/platform/httpserver"
)

// TestChain_PreservesHijacker guards against a regression where Chain's logging middleware wraps
// http.ResponseWriter in a way that drops the http.Hijacker interface — coder/websocket's
// Accept (used by GET /api/stream) needs Hijack and fails every WebSocket upgrade with
// 501 Not Implemented if it's missing.
func TestChain_PreservesHijacker(t *testing.T) {
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
	hijackErrCh := make(chan error, 1)
	srv.HandleFunc("GET /hijack", func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := http.NewResponseController(w).Hijack()
		hijackErrCh <- err
		if err == nil {
			conn.Close()
		}
	})

	done := make(chan error, 1)
	go func() { done <- srv.Start() }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	waitForServer(t, addr)

	//nolint:bodyclose // the handler hijacks the connection; there is no HTTP response to close.
	_, _ = http.Get("http://" + addr + "/hijack")

	select {
	case err := <-hijackErrCh:
		if err != nil {
			t.Fatalf("Hijack through Chain's middleware: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler never ran")
	}
}
