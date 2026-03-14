package httpkit

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewServer_Defaults(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.port != ":8989" {
		t.Errorf("expected default port :8989, got %s", s.port)
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
	dir := t.TempDir()

	certFile, err := os.CreateTemp(dir, "cert*.pem")
	if err != nil {
		t.Fatal(err)
	}
	if err = certFile.Close(); err != nil {
		t.Fatal(err)
	}

	keyFile, err := os.CreateTemp(dir, "key*.pem")
	if err != nil {
		t.Fatal(err)
	}
	if err = keyFile.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(WithTLS(certFile.Name(), keyFile.Name()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.certFile != certFile.Name() {
		t.Errorf("certFile not set: got %q", s.certFile)
	}
	if s.keyFile != keyFile.Name() {
		t.Errorf("keyFile not set: got %q", s.keyFile)
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
	err = s.Start()
	if err == nil {
		t.Fatal("expected error when starting with no routes")
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
