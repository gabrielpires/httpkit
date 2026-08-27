# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-08-27

### Changed
- **Breaking for consumers:** minimum supported Go version is now 1.26 (`go 1.26.0` in `go.mod`). Go 1.25 has reached end of life; the package is built and tested against both currently supported releases, 1.26 and 1.27
- Trailing-slash and path-cleaning redirects issued by the underlying `http.ServeMux` now return `307 Temporary Redirect` instead of `301 Moved Permanently`. This is an unconditional Go 1.26 standard library change, so clients that cached the previous permanent redirects may need to be flushed
- Servers created with `WithTLS` and `WithSelfAssignedCert` now also offer the `SecP256r1MLKEM768` and `SecP384r1MLKEM1024` post-quantum hybrid key exchanges, following the Go 1.26 `tlssecpmlkem` default. Post-quantum key exchange itself is not new: `X25519MLKEM768` has been offered since Go 1.24 and remains what most clients negotiate. httpkit leaves `Config.CurvePreferences` unset, so the standard library defaults apply
- URL parsing is stricter about colons in request URLs, following the Go 1.26 `urlstrictcolons` default
- `buildChain` iterates the middleware slice with `slices.Backward`. Behavior is unchanged
- CI tests on Go 1.26 and stable, lints on Go 1.27 with golangci-lint v2.13.1, and uses refreshed action versions (checkout v5, setup-go v6, codecov v5, golangci-lint-action v9, action-gh-release v3)
- CI coverage upload now authenticates with `CODECOV_TOKEN` and fails the job on upload errors instead of passing silently, which is why no coverage had reached Codecov since the v0.1.0 release
- README documents the Go version requirement and replaces the Go Report Card badge, which was sunset upstream and renders as "retired", with a Go version badge sourced from `go.mod`

## [0.1.0] - 2026-03-14

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

[0.2.0]: https://github.com/gabrielpires/httpkit/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/gabrielpires/httpkit/releases/tag/v0.1.0
