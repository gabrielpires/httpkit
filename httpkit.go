// Package httpkit provides ergonomic HTTP server utilities with support for
// TLS, self-assigned certificates, and functional options configuration.
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
	port         string
	certFile     string
	keyFile      string
	selfAssigned *tls.Config
	httpServer   *http.Server
}

// Handle registers the handler for the given path pattern.
func (s *Server) Handle(path string, handler http.Handler) {
	s.routes = append(s.routes, handler)
	s.mux.Handle(path, handler)
}

// Start begins listening and serving HTTP or HTTPS requests.
// The provided context controls graceful shutdown — cancelling it will drain
// in-flight requests and stop the server. Returns an error if no routes have
// been registered or if the server fails to start.
func (s *Server) Start(ctx context.Context) error {
	if len(s.routes) == 0 {
		return errors.New("no routes configured. use s.Handle(path string, handler http.Handler) function to specify it before starting the server")
	}

	srv := &http.Server{
		Addr:    s.port,
		Handler: s.mux,
	}

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
