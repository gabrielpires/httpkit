package httpkit_test

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gabrielpires/httpkit"
)

func ExampleNewServer() {
	s, err := httpkit.NewServer(
		httpkit.WithPort(":8080"),
	)
	if err != nil {
		log.Fatal(err)
	}

	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hello")) //nolint:errcheck
	}))

	_ = s
}

func ExampleNewServer_tls() {
	s, err := httpkit.NewServer(
		httpkit.WithPort(":8443"),
		httpkit.WithTLS("cert.pem", "key.pem"),
	)
	if err != nil {
		log.Fatal(err)
	}

	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hello")) //nolint:errcheck
	}))

	_ = s
}

func ExampleNewServer_devTLS() {
	s, err := httpkit.NewServer(
		httpkit.WithSelfAssignedCert(),
	)
	if err != nil {
		log.Fatal(err)
	}

	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hello")) //nolint:errcheck
	}))

	_ = s
}

func ExampleServer_Stop() {
	s, err := httpkit.NewServer()
	if err != nil {
		log.Fatal(err)
	}

	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	go func() {
		if err := s.Start(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()

	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Stop(stopCtx); err != nil {
		log.Fatal(err)
	}
}

func ExampleServer_Middleware() {
	s, err := httpkit.NewServer()
	if err != nil {
		log.Fatal(err)
	}

	s.Middleware(httpkit.RequestID)
	s.Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := httpkit.RequestIDFromContext(r.Context())
			w.Header().Set("X-Trace-ID", id)
			next.ServeHTTP(w, r)
		})
	})

	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	_ = s
}

func ExampleRequestIDFromContext() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := httpkit.RequestIDFromContext(r.Context())
		w.Header().Set("X-Request-ID", id)
		w.WriteHeader(http.StatusOK)
	})

	_ = handler
}

func ExampleWithReadTimeout() {
	s, err := httpkit.NewServer(
		httpkit.WithReadTimeout(5*time.Second),
		httpkit.WithWriteTimeout(10*time.Second),
		httpkit.WithIdleTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	_ = s
}

func ExampleWithServerConfig() {
	s, err := httpkit.NewServer(
		httpkit.WithServerConfig(func(srv *http.Server) {
			srv.MaxHeaderBytes = 1 << 20
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	_ = s
}
