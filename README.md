# httpkit

[![Go Reference](https://pkg.go.dev/badge/github.com/gabrielpires/httpkit.svg)](https://pkg.go.dev/github.com/gabrielpires/httpkit)
[![CI](https://github.com/gabrielpires/httpkit/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/gabrielpires/httpkit/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/gabrielpires/httpkit)](https://goreportcard.com/report/github.com/gabrielpires/httpkit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

After building several REST APIs in Go, I kept reaching for the same boilerplate — setting up `net/http`, wiring TLS, handling graceful shutdown. Rather than pulling in a full framework, I wanted to explore what the stdlib could do on its own. So I built a small server wrapper, used it across my own projects, iterated on it, and eventually decided to make it official.

Before going public, I compared what I had against packages like chi and goji, filled the gaps, and shaped it into something I'd be happy to depend on long-term. That's httpkit — a small, opinionated Go library that wraps `net/http` to give you a zero-config HTTP/HTTPS server with sane defaults and TLS auto-detection, without leaving the stdlib behind.

## Why httpkit

| Feature | chi | goji | httpkit |
|---|---|---|---|
| stdlib compatible | Yes | Yes | Yes |
| Zero external dependencies | No | No | Yes |
| TLS (file-based) | Via stdlib | Via stdlib | Built-in |
| Self-signed cert (dev) | No | No | Built-in |
| Graceful shutdown | Manual | Built-in | Built-in |
| Context-based shutdown | Manual | Yes | Yes |
| Explicit `Stop(ctx)` | No | No | Yes |
| Functional options config | No | No | Yes |
| Global middleware chain | Yes | Yes | Yes |
| Request ID middleware | No | No | Built-in |
| Health check endpoint | No | No | Built-in (`/healthcheck`) |
| Timeout configuration | Manual | Manual | Built-in |

## Install

```bash
go get github.com/gabrielpires/httpkit
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/gabrielpires/httpkit"
)

func main() {
    s, err := httpkit.NewServer(
        httpkit.WithPort(":8080"),
    )
    if err != nil {
        log.Fatal(err)
    }

    s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("hello"))
    }))

    if err = s.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

## Graceful Shutdown

Pass a cancellable context to `Start` — cancelling it drains in-flight requests before stopping. Use `Stop` when you need explicit control over the shutdown timeout.

```go
ctx, cancel := context.WithCancel(context.Background())

// cancel on SIGTERM/SIGINT
go func() {
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    <-sig
    cancel()
}()

s.Start(ctx)

// or stop explicitly with a timeout
stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer stopCancel()
s.Stop(stopCtx)
```

## TLS

**File-based** — provide your own certificate and key:

```go
s, err := httpkit.NewServer(
    httpkit.WithTLS("cert.pem", "key.pem"),
)
```

**Self-signed** — generates an in-memory ECDSA certificate at startup. Suitable for development and internal tooling only; browsers will show a security warning.

```go
s, err := httpkit.NewServer(
    httpkit.WithSelfAssignedCert(),
)
```

## Middleware

Register global middleware with `Middleware`. All middlewares are applied to every route in declaration order, outermost first, regardless of where `Handle` calls appear.

```go
s.Middleware(httpkit.RequestID)
s.Middleware(loggingMiddleware)

s.Handle("/", handlers.Home)
```

For per-route middleware, wrap the handler directly:

```go
s.Handle("/admin", authMiddleware(handlers.Admin))
```

### Built-in: RequestID

Assigns a unique ID to each request. Reuses the `X-Request-ID` header if already set by an upstream proxy. The ID is written to the response header and available in the context.

```go
s.Middleware(httpkit.RequestID)

// inside a handler
id := httpkit.RequestIDFromContext(r.Context())
```

## Options Reference

| Option | Description |
|---|---|
| `WithPort(":8080")` | Listening port. Format `:n`, range 1–65535. Default: `:8080` |
| `WithTLS(cert, key)` | Enable TLS using certificate and key file paths |
| `WithSelfAssignedCert()` | Generate an in-memory self-signed cert (dev only) |
| `WithReadTimeout(d)` | Max duration to read the full request. Zero = no timeout |
| `WithWriteTimeout(d)` | Max duration to write the response. Zero = no timeout |
| `WithIdleTimeout(d)` | Max idle time between keep-alive requests. Zero = no timeout |
| `WithServerConfig(fn)` | Direct access to the underlying `http.Server` for advanced configuration. `Addr`, `Handler`, and `TLSConfig` are managed by httpkit and will be overwritten |

## License

[MIT](LICENSE) © 2026 [Gabriel Pires](https://gabrielpires.com.br)
