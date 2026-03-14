# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Global middleware chain via `Middleware` method — applied to all routes at `Start` time, outermost first
- Built-in `RequestID` middleware — generates a unique per-request ID, propagates upstream `X-Request-ID`, and exposes `RequestIDFromContext` helper
- `WithServerConfig(fn)` option for direct access to the underlying `http.Server`
- `WithReadTimeout`, `WithWriteTimeout`, `WithIdleTimeout` options for timeout configuration
- Default `/healthcheck` endpoint returning `200 OK` on every server
- `Stop(ctx)` for explicit graceful shutdown with caller-controlled timeout
- Context-based graceful shutdown in `Start(ctx)` — cancelling the context drains in-flight requests
- `WithSelfAssignedCert()` option — generates an in-memory ECDSA self-signed certificate for development use
- `WithTLS(cert, key)` option — file-based TLS with eager validation at construction time
- `WithPort(port)` option — port format and range validation at construction time
- Functional options pattern via `NewServer(opts ...Option)`
- Test fixtures in `testdata/valid` and `testdata/invalid` for TLS tests
- CI pipeline with golangci-lint v2 and Go matrix testing

### Changed
- Rewrote public API — replaced exported struct fields and setters with functional options
- `Start` no longer panics on error — returns `error` consistently
- Unified HTTP, file-based TLS, and self-assigned TLS paths into a single `http.Server` construction in `Start`
- Default port changed from `:8443` to `:8080`
- All exported symbols now carry doc comments

### Fixed
- Data race between `Start` and `Stop` on `httpServer` field — protected with `sync.Mutex`
- `slog` call with invalid map syntax in `Start`

[Unreleased]: https://github.com/gabrielpires/httpkit/compare/HEAD...HEAD
