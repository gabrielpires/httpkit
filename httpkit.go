// Package httpkit provides a small, opinionated HTTP/HTTPS server built on
// top of [net/http]. It offers a functional options API, built-in TLS support,
// context-based graceful shutdown, a global middleware chain, and sensible
// defaults — with no external dependencies.
//
// # Basic usage
//
//	s, err := httpkit.NewServer(httpkit.WithPort(":8080"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	s.Handle("/hello", myHandler)
//	s.Start(ctx)
//
// # TLS
//
// Use [WithTLS] for file-based certificates or [WithSelfAssignedCert] for an
// in-memory self-signed certificate during development.
//
//	s, err := httpkit.NewServer(httpkit.WithTLS("cert.pem", "key.pem"))
//
// # Graceful shutdown
//
// Pass a cancellable context to [Server.Start]. When cancelled, in-flight
// requests are drained before the server stops. Use [Server.Stop] for
// explicit control over the shutdown timeout.
//
//	ctx, cancel := context.WithCancel(context.Background())
//	go s.Start(ctx)
//	// later...
//	cancel()
//
// # Middleware
//
// Register global middleware with [Server.Middleware]. For per-route
// middleware, wrap the handler directly in [Server.Handle]:
//
//	s.Middleware(httpkit.RequestID)
//	s.Handle("/admin", authMiddleware(adminHandler))
package httpkit

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Option is a functional option for configuring a Server.
type Option func(s *Server) error

// Server is an HTTP server with optional TLS support.
type Server struct {
	mu           sync.Mutex
	routes       []http.Handler
	mux          *http.ServeMux
	middlewares  []func(http.Handler) http.Handler
	port         string
	certFile     string
	keyFile      string
	selfAssigned *tls.Config
	httpServer   *http.Server
	readTimeout  time.Duration
	writeTimeout time.Duration
	idleTimeout  time.Duration
	serverConfig func(*http.Server)
}

// Handle registers the handler for the given path pattern.
func (s *Server) Handle(path string, handler http.Handler) {
	s.routes = append(s.routes, handler)
	s.mux.Handle(path, handler)
}

// Middleware appends a middleware to the global chain. All middlewares are
// applied to every route regardless of declaration order, and are evaluated at
// Start time. For selective middleware, wrap the handler directly in Handle:
//
//	server.Handle("/admin", authMiddleware(handlers.Admin))
func (s *Server) Middleware(mw func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, mw)
}

// Start begins listening and serving HTTP or HTTPS requests.
// The provided context controls graceful shutdown — cancelling it will drain
// in-flight requests and stop the server. Returns an error if no routes have
// been registered or if the server fails to start.
func (s *Server) Start(ctx context.Context) error {
	if len(s.routes) == 0 {
		return errors.New("no routes configured. use s.Handle(path string, handler http.Handler) function to specify it before starting the server")
	}

	handler := buildChain(s.mux, s.middlewares)

	srv := &http.Server{
		Addr:         s.port,
		Handler:      handler,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
		IdleTimeout:  s.idleTimeout,
	}

	if s.serverConfig != nil {
		s.serverConfig(srv)
	}

	// ensure Start-owned fields are not overwritten by serverConfig
	srv.Addr = s.port
	srv.Handler = handler

	s.mu.Lock()
	s.httpServer = srv
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	var err error
	switch {
	case s.selfAssigned != nil:
		srv.TLSConfig = s.selfAssigned
		slog.Info("https self-assigned server starting", "port", s.port)
		err = srv.ListenAndServeTLS("", "")
	case s.certFile != "":
		slog.Info("https server starting", "port", s.port, "cert", s.certFile, "key", s.keyFile)
		err = srv.ListenAndServeTLS(s.certFile, s.keyFile)
	default:
		slog.Info("http server starting", "port", s.port)
		err = srv.ListenAndServe()
	}

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Stop gracefully shuts down the server, waiting for in-flight requests to
// complete until the provided context is cancelled or times out.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpServer
	s.mu.Unlock()

	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// NewServer creates a new Server applying the provided options.
// Returns an error if any option fails validation.
func NewServer(opts ...Option) (*Server, error) {
	s := &Server{
		mux:  http.NewServeMux(),
		port: ":8080",
	}

	s.mux.HandleFunc("/healthcheck", s.healthcheck)

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Server) healthcheck(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// WithServerConfig provides direct access to the underlying http.Server for
// advanced configuration. The callback is applied before the server starts.
// Addr, Handler, and TLSConfig are managed by httpkit and will be overwritten.
func WithServerConfig(fn func(*http.Server)) Option {
	return func(s *Server) error {
		s.serverConfig = fn
		return nil
	}
}

// WithReadTimeout sets the maximum duration for reading the entire request,
// including the body. A zero value means no timeout.
func WithReadTimeout(d time.Duration) Option {
	return func(s *Server) error {
		if d < 0 {
			return fmt.Errorf("read timeout must be non-negative, got %v", d)
		}
		s.readTimeout = d
		return nil
	}
}

// WithWriteTimeout sets the maximum duration before timing out writes of the
// response. A zero value means no timeout.
func WithWriteTimeout(d time.Duration) Option {
	return func(s *Server) error {
		if d < 0 {
			return fmt.Errorf("write timeout must be non-negative, got %v", d)
		}
		s.writeTimeout = d
		return nil
	}
}

// WithIdleTimeout sets the maximum amount of time to wait for the next request
// when keep-alives are enabled. A zero value means no timeout.
func WithIdleTimeout(d time.Duration) Option {
	return func(s *Server) error {
		if d < 0 {
			return fmt.Errorf("idle timeout must be non-negative, got %v", d)
		}
		s.idleTimeout = d
		return nil
	}
}

// WithPort sets the listening port. The port must be in the format ":n" where n is between 1 and 65535.
func WithPort(port string) Option {
	return func(s *Server) error {
		var portRegex = regexp.MustCompile(`^:(\d{1,5})$`)
		m := portRegex.FindStringSubmatch(port)
		if m == nil {
			return fmt.Errorf("invalid port format %q: expected \":port\" (e.g. \":8080\")", port)
		}

		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > 65535 {
			return fmt.Errorf("port %d out of range [1, 65535]", n)
		}

		s.port = port
		return nil
	}
}

// WithTLS configures the server to use TLS with the provided certificate and key file paths.
// Both files must exist at the time the server is created.
func WithTLS(certFile, keyFile string) Option {
	return func(s *Server) error {
		if len(certFile) == 0 {
			return errors.New("certFile path is empty")
		}

		if len(keyFile) == 0 {
			return errors.New("keyFile path is empty")
		}

		if _, err := os.Stat(certFile); err != nil {
			return fmt.Errorf("certFile: %w", err)
		}

		if _, err := os.Stat(keyFile); err != nil {
			return fmt.Errorf("keyFile: %w", err)
		}

		s.certFile = certFile
		s.keyFile = keyFile
		return nil
	}
}

// WithSelfAssignedCert generates an in-memory self-signed ECDSA certificate for development use.
// The certificate is valid for one year. Not suitable for production.
func WithSelfAssignedCert() Option {
	return func(s *Server) error {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return err
		}

		template := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject:      pkix.Name{Organization: []string{"httpkit (dev)"}},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}

		certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
		if err != nil {
			return err
		}

		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return err
		}

		cert, err := tls.X509KeyPair(
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
			pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		)
		if err != nil {
			return err
		}

		s.selfAssigned = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		return nil
	}
}

// buildChain wraps handler with middlewares in order, so the first middleware
// in the slice is the outermost (executes first).
func buildChain(h http.Handler, middlewares []func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
