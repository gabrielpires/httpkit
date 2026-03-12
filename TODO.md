# httpkit — Investigation Report & TODO

---

## Part 1: Ecosystem Landscape

| Library | Stars | stdlib compatible | TLS | Graceful shutdown | Routing | Middleware |
|---|---|---|---|---|---|---|
| Gin | ~80k | Wraps (own Context) | Via stdlib | Manual | Radix tree | Yes |
| Fiber | ~35k | No (fasthttp) | Built-in | Built-in | Radix tree | Yes |
| Echo | ~30k | Wraps (own Context) | Built-in + AutoTLS | Built-in | Radix tree | Yes |
| gorilla/mux | ~21k | Full compat | Via stdlib | Manual | Pattern match | Yes — **archived** |
| chi | ~18k | Full compat | Via stdlib | Manual | Radix tree | Yes |
| httprouter | ~17k | Partial | Via stdlib | Manual | Radix tree | No |
| negroni | ~7.5k | Full compat | Via stdlib | Manual | None | Yes |
| goji | ~3k | Full compat | Via stdlib | **Built-in** | Pattern | Yes |

**Closest to httpkit in scope:** chi (minimal, stdlib-compatible) and goji (minimal + graceful shutdown).

---

## Part 2: Go Standards Compliance

### High severity

- [ ] **`Start()` panics instead of returning an error** — libraries must never `panic` on operational errors. `Start` should return `error`.
- [ ] **`SetCertificate` silently swallows all errors** — a missing file and an empty string produce identical results (`nil`). Should return `error`.
- [ ] **`fileExists` uses deprecated `os.IsNotExist`** — since Go 1.13 the idiomatic form is `errors.Is(err, fs.ErrNotExist)`.

### Medium severity

- [ ] **Three competing ways to configure the same thing** — port can be set via direct field assignment (`server.Port = ...`), via `SetPort(...)`, or via `Options` passed to `Start`. Consolidate into a constructor with functional options or a config struct:
  ```go
  // functional options (most idiomatic)
  NewServer(WithPort(":8080"), WithTLS(cert, key))
  // or config struct passed to constructor
  NewServer(Options{Port: ":8080"})
  ```
- [ ] **`Server.ServerMux`, `Server.Port`, `Server.Certificate` should be unexported** — exporting mutable fields bypasses setters and makes state unpredictable.
- [ ] **`handlers []http.Handler` is dead code** — appended to in `AddHandler` but never read; routing is done exclusively via `ServerMux`. Remove it.
- [ ] **`GenericHandler` should be unexported** — it is a fallback implementation detail, not a public API. Rename to `defaultHandler`.
- [ ] **No doc comments on any exported symbol** — every exported type, field, and function needs a doc comment for `go doc` and `pkg.go.dev`.

### Low severity

- [ ] `SetPort`/`SetCertificate` return values callers never use — idiomatic setters don't return values
- [ ] `fileExists` parameter named `filepath` shadows the stdlib `path/filepath` package — rename to `path` or `filePath`
- [ ] Default port `:8443` is TLS convention but used for plain HTTP when no cert is set — confusing default
- [ ] Log says `"http server started"` before the bind succeeds — should say `"starting"` or move log after confirmed bind
- [ ] `TestSetPort_Setted` — "Setted" is not a word; use `TestSetPort_CustomValue` or similar

---

## Part 3: Test Coverage

| Function | Coverage | Issue |
|---|---|---|
| `SetPort` | ~100% | Good |
| `GenericHandler.ServeHTTP` | ~100% | Good |
| `SetCertificate` | ~60% | `TestSetCertificate_Setted` depends on `testing.pem` existing — will fail on clean checkout |
| `AddHandler` | ~70% | Called but `handlers` slice never asserted |
| `NewServer` | ~50% | Called, output not verified |
| `Start` | **0%** | Never called in any test |
| `fileExists` | **~20%** | Only exercised via a broken test |

**Overall: ~50% meaningful coverage — insufficient for a published library.**

- [ ] `TestStartAndRouting` is an empty test — no call to `Start`, no assertions; fix or remove
- [ ] `TestSetCertificate_Setted` is environment-dependent (requires `testing.pem`) — will fail on CI; add fixture to `testdata/` or mock the file check
- [ ] Convert multiple `TestXxx_Case` functions to table-driven tests with `t.Run` subtests
- [ ] Replace `t.Errorf` with `t.Fatalf` where subsequent assertions are meaningless if the first fails
- [ ] Add tests for `Start` using `httptest.NewServer` (HTTP path, HTTPS path, auto-fallback handler)
- [ ] Add direct tests for `fileExists` (existing file, missing file, permission error)

---

## Part 4: Missing Features

### Must-have to be a credible library

- [ ] `Start()` returns `error` instead of panicking
- [ ] Graceful shutdown — `Shutdown(ctx context.Context)` or signal handling (`SIGINT`/`SIGTERM`)
- [ ] `context.Context` support in `Start` for coordinated shutdown
- [ ] Doc comments on all exported symbols
- [ ] Clean pass under `go vet` and `golangci-lint`

### Should-have for good DX

- [ ] Functional options or clean constructor-based config (eliminate competing config paths)
- [ ] Timeout configuration — read timeout, write timeout, idle timeout on `http.Server`
- [ ] Expose or allow configuration of the underlying `http.Server`
- [ ] Consistent `error` returns across all public methods

### Optional / to consider

- [ ] Automatic TLS via Let's Encrypt (Echo has `StartAutoTLS`; Caddy is the gold standard)
- [ ] Middleware chain support (chi/negroni pattern)
- [ ] HTTP/2 support (enabled automatically by `http.Server` with TLS)
- [ ] Request ID / structured access log middleware
- [ ] Health check endpoint
- [ ] Graceful shutdown with configurable timeout (goji does this well with a single `Serve(ctx)`)
- [ ] Dual listener: HTTP redirect + HTTPS on separate ports

---

## Niche to own

The gap that would make httpkit genuinely useful — the thing chi and goji don't provide out of the box — is **opinionated TLS + graceful shutdown in one `Start(ctx)` / `Stop(ctx)` call**. That is the niche worth owning.
