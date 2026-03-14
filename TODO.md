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

## Part 2: Missing Features

### Should-have for good DX

- [ ] Timeout configuration — `WithReadTimeout`, `WithWriteTimeout`, `WithIdleTimeout` on the underlying `http.Server`
- [ ] Expose or allow configuration of the underlying `http.Server`

### Optional / to consider

- [ ] Automatic TLS via Let's Encrypt (Echo has `StartAutoTLS`; Caddy is the gold standard)
- [ ] Middleware chain support (chi/negroni pattern)
- [ ] HTTP/2 support (enabled automatically by `http.Server` with TLS)
- [ ] Request ID / structured access log middleware
- [ ] Dual listener: HTTP redirect + HTTPS on separate ports

---

## Niche to own

The gap that would make httpkit genuinely useful — the thing chi and goji don't provide out of the box — is **opinionated TLS + graceful shutdown in one `Start(ctx)` / `Stop(ctx)` call**. ✅ Achieved.
