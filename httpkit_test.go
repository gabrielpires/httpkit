package httpkit

import (
	"context"
	"crypto/tls"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// freePort finds an available TCP port and returns it in ":port" format.
func freePort(t *testing.T) string {
	t.Helper()
	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	if err = ln.Close(); err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return ":" + port
}

// waitForServer retries a TCP dial until the server is accepting connections or the deadline is exceeded.
func waitForServer(t *testing.T, addr string) {
	t.Helper()
	dialer := &net.Dialer{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := dialer.DialContext(context.Background(), "tcp", addr)
		if err == nil {
			if err = conn.Close(); err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s did not start in time", addr)
}

func TestNewServer_Defaults(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.port != ":8080" {
		t.Errorf("expected default port :8080, got %s", s.port)
	}
	if s.mux == nil {
		t.Error("expected mux to be initialized")
	}
}

func TestNewServer_OptionError(t *testing.T) {
	badOpt := func(_ *Server) error {
		return errors.New("bad option")
	}
	s, err := NewServer(badOpt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if s != nil {
		t.Error("expected nil server when option fails")
	}
}

func TestWithPort(t *testing.T) {
	cases := []struct {
		port    string
		wantErr bool
	}{
		{":8080", false},
		{":1", false},
		{":65535", false},
		{":0", true},
		{":65536", true},
		{"8080", true},
		{"", true},
		{":abc", true},
		{"::8080", true},
	}

	for _, tc := range cases {
		t.Run(tc.port, func(t *testing.T) {
			_, err := NewServer(WithPort(tc.port))
			if tc.wantErr && err == nil {
				t.Errorf("expected error for port %q, got nil", tc.port)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for port %q: %v", tc.port, err)
			}
		})
	}
}

func TestWithTLS_EmptyCertFile(t *testing.T) {
	_, err := NewServer(WithTLS("", "key.pem"))
	if err == nil {
		t.Fatal("expected error for empty certFile")
	}
}

func TestWithTLS_EmptyKeyFile(t *testing.T) {
	_, err := NewServer(WithTLS("cert.pem", ""))
	if err == nil {
		t.Fatal("expected error for empty keyFile")
	}
}

func TestWithTLS_NonExistentCertFile(t *testing.T) {
	_, err := NewServer(WithTLS("nonexistent_cert.pem", "nonexistent_key.pem"))
	if err == nil {
		t.Fatal("expected error for nonexistent certFile")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestWithTLS_NonExistentKeyFile(t *testing.T) {
	certFile, err := os.CreateTemp(t.TempDir(), "cert*.pem")
	if err != nil {
		t.Fatal(err)
	}
	if err = certFile.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = NewServer(WithTLS(certFile.Name(), "nonexistent_key.pem"))
	if err == nil {
		t.Fatal("expected error for nonexistent keyFile")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestWithTLS_ValidFiles(t *testing.T) {
	cert := "testdata/valid/cert.pem"
	key := "testdata/valid/key.pem"

	s, err := NewServer(WithTLS(cert, key))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.certFile != cert {
		t.Errorf("certFile not set: got %q", s.certFile)
	}
	if s.keyFile != key {
		t.Errorf("keyFile not set: got %q", s.keyFile)
	}
}

func TestStart_WithTLS_ValidCert(t *testing.T) {
	s, err := NewServer(WithTLS("testdata/valid/cert.pem", "testdata/valid/key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	s.port = freePort(t)
	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start(ctx)
	}()

	// wait for TLS port to be open then connect with InsecureSkipVerify
	// since the cert is self-signed
	waitForServer(t, s.port)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://localhost"+s.port+"/", nil)
	if err != nil {
		cancel()
		t.Fatalf("unexpected error building request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("unexpected error: %v", err)
	}
	if err = resp.Body.Close(); err != nil {
		cancel()
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	cancel()
	if err = <-errCh; err != nil {
		t.Errorf("expected clean shutdown, got %v", err)
	}
}

func TestStart_WithTLS_InvalidCert(t *testing.T) {
	s, err := NewServer(WithTLS("testdata/invalid/cert.pem", "testdata/invalid/key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	s.port = freePort(t)
	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	err = s.Start(context.Background())
	if err == nil {
		t.Fatal("expected error starting server with invalid cert")
	}
}

func TestStart_WithSelfAssignedCert_GracefulShutdown(t *testing.T) {
	s, err := NewServer(WithSelfAssignedCert())
	if err != nil {
		t.Fatal(err)
	}
	s.port = freePort(t)
	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start(ctx)
	}()

	waitForServer(t, s.port)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://localhost"+s.port+"/", nil)
	if err != nil {
		cancel()
		t.Fatalf("unexpected error building request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("unexpected error: %v", err)
	}
	if err = resp.Body.Close(); err != nil {
		cancel()
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	cancel()
	if err = <-errCh; err != nil {
		t.Errorf("expected clean shutdown, got %v", err)
	}
}

func TestMiddleware_MiddlewareExecutionOrder(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	var order []string

	s.Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "first")
			next.ServeHTTP(w, r)
		})
	})
	s.Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "second")
			next.ServeHTTP(w, r)
		})
	})

	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}))

	chain := buildChain(s.mux, s.middlewares)
	runner := httptest.NewServer(chain)
	defer runner.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, runner.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	expected := []string{"first", "second", "handler"}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestMiddleware_MiddlewareCanShortCircuit(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	s.Middleware(func(_ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	})

	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	chain := buildChain(s.mux, s.middlewares)
	runner := httptest.NewServer(chain)
	defer runner.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, runner.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestWithServerConfig(t *testing.T) {
	s, err := NewServer(WithServerConfig(func(srv *http.Server) {
		srv.MaxHeaderBytes = 1 << 20
	}))
	if err != nil {
		t.Fatal(err)
	}
	if s.serverConfig == nil {
		t.Fatal("expected serverConfig to be set")
	}
}

func TestWithServerConfig_OwnedFieldsNotOverwritten(t *testing.T) {
	s, err := NewServer(
		WithPort(":9090"),
		WithServerConfig(func(srv *http.Server) {
			srv.Addr = ":1111"
			srv.Handler = nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	s.port = freePort(t)
	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start(ctx)
	}()

	waitForServer(t, s.port)

	s.mu.Lock()
	addr := s.httpServer.Addr
	handler := s.httpServer.Handler
	s.mu.Unlock()

	if addr != s.port {
		t.Errorf("expected Addr %q, got %q", s.port, addr)
	}
	if handler == nil {
		t.Error("expected Handler to be set, got nil")
	}

	cancel()
	if err = <-errCh; err != nil {
		t.Errorf("expected clean shutdown, got %v", err)
	}
}

func TestWithReadTimeout(t *testing.T) {
	cases := []struct {
		d       time.Duration
		wantErr bool
	}{
		{5 * time.Second, false},
		{0, false},
		{-1 * time.Second, true},
	}
	for _, tc := range cases {
		_, err := NewServer(WithReadTimeout(tc.d))
		if tc.wantErr && err == nil {
			t.Errorf("expected error for read timeout %v, got nil", tc.d)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("unexpected error for read timeout %v: %v", tc.d, err)
		}
	}
}

func TestWithWriteTimeout(t *testing.T) {
	cases := []struct {
		d       time.Duration
		wantErr bool
	}{
		{5 * time.Second, false},
		{0, false},
		{-1 * time.Second, true},
	}
	for _, tc := range cases {
		_, err := NewServer(WithWriteTimeout(tc.d))
		if tc.wantErr && err == nil {
			t.Errorf("expected error for write timeout %v, got nil", tc.d)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("unexpected error for write timeout %v: %v", tc.d, err)
		}
	}
}

func TestWithIdleTimeout(t *testing.T) {
	cases := []struct {
		d       time.Duration
		wantErr bool
	}{
		{30 * time.Second, false},
		{0, false},
		{-1 * time.Second, true},
	}
	for _, tc := range cases {
		_, err := NewServer(WithIdleTimeout(tc.d))
		if tc.wantErr && err == nil {
			t.Errorf("expected error for idle timeout %v, got nil", tc.d)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("unexpected error for idle timeout %v: %v", tc.d, err)
		}
	}
}

func TestTimeouts_AppliedToServer(t *testing.T) {
	s, err := NewServer(
		WithReadTimeout(1*time.Second),
		WithWriteTimeout(2*time.Second),
		WithIdleTimeout(3*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	if s.readTimeout != 1*time.Second {
		t.Errorf("expected readTimeout 1s, got %v", s.readTimeout)
	}
	if s.writeTimeout != 2*time.Second {
		t.Errorf("expected writeTimeout 2s, got %v", s.writeTimeout)
	}
	if s.idleTimeout != 3*time.Second {
		t.Errorf("expected idleTimeout 3s, got %v", s.idleTimeout)
	}
}

func TestWithSelfAssignedCert(t *testing.T) {
	s, err := NewServer(WithSelfAssignedCert())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.selfAssigned == nil {
		t.Fatal("expected selfAssigned tls.Config to be set")
	}
	if len(s.selfAssigned.Certificates) == 0 {
		t.Error("expected at least one certificate in tls.Config")
	}
}

func TestStart_NoRoutes(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	err = s.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when starting with no routes")
	}
}

func TestStop_ServerNotStarted(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Stop(context.Background()); err != nil {
		t.Errorf("expected nil stopping unstarted server, got %v", err)
	}
}

func TestStart_ContextCancellation(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	s.port = freePort(t)
	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start(ctx)
	}()

	waitForServer(t, s.port)
	cancel()

	if err = <-errCh; err != nil {
		t.Errorf("expected clean shutdown on context cancel, got %v", err)
	}
}

func TestStop_StopsRunningServer(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	s.port = freePort(t)
	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start(context.Background())
	}()

	waitForServer(t, s.port)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = s.Stop(stopCtx); err != nil {
		t.Errorf("unexpected error stopping server: %v", err)
	}

	if err = <-errCh; err != nil {
		t.Errorf("expected clean shutdown, got %v", err)
	}
}

func TestHealthcheck(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	runner := httptest.NewServer(s.mux)
	defer runner.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, runner.URL+"/healthcheck", nil)
	if err != nil {
		t.Fatalf("unexpected error building request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandle_MuxReceivesRequest(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	s.Handle("/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	runner := httptest.NewServer(s.mux)
	defer runner.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, runner.URL+"/hello", nil)
	if err != nil {
		t.Fatalf("unexpected error building request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandle_PopulatesRoutes(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	if len(s.routes) != 0 {
		t.Fatalf("expected 0 routes before any Handle call, got %d", len(s.routes))
	}

	s.Handle("/a", http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	s.Handle("/b", http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	if len(s.routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(s.routes))
	}
}
