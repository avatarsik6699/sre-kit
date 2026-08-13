// Package httpserver owns HTTP server bootstrap and the single mount point every bounded context
// registers its interfaces/http handlers on.
package httpserver

import (
	"context"
	"net/http"
	"time"
)

// Server wraps a *http.ServeMux (the mount point) and the net/http server that serves it through
// the shared middleware chain.
type Server struct {
	Mux        *http.ServeMux
	addr       string
	httpServer *http.Server
	middleware []func(http.Handler) http.Handler
}

// New creates a Server listening on addr (e.g. ":8080"). Modules mount their routes on Mux before
// Start is called.
func New(addr string) *Server {
	return &Server{Mux: http.NewServeMux(), addr: addr}
}

// Use registers additional middleware (e.g. internal/auth's session-required check) around Mux,
// inside the standard request-id/logging/recover chain — so even a rejected request still gets
// logged. Middleware added first wraps closest to Mux.
func (s *Server) Use(mw func(http.Handler) http.Handler) {
	s.middleware = append(s.middleware, mw)
}

// Handle registers a handler for pattern on the shared mux (Go 1.22+ method+path patterns, e.g.
// "GET /api/sources").
func (s *Server) Handle(pattern string, handler http.Handler) {
	s.Mux.Handle(pattern, handler)
}

// HandleFunc is the http.HandlerFunc convenience form of Handle.
func (s *Server) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.Mux.HandleFunc(pattern, handler)
}

// Start blocks serving HTTP on addr with the middleware chain (request ID, logging, recover)
// applied around the mux.
func (s *Server) Start() error {
	var handler http.Handler = s.Mux
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i](handler)
	}
	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           Chain(handler),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops serving. Kept for the composition root's signal-handling shutdown path.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
