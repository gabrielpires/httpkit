# httpkit — Claude Code Context

## Project

Go package providing ergonomic HTTP utilities.

Module: `github.com/gabrielpires/httpkit`
Go version: 1.25

## Commands

```bash
make test      # run tests with race detector
make lint      # run golangci-lint
make fmt       # gofmt + goimports
make tidy      # go mod tidy
make coverage  # generate coverage report
```

## Layout

| Path | Purpose |
|------|---------|
| `*.go` (root) | Public API |
| `internal/` | Private helpers, not exported |
| `testdata/` | Test fixtures |

## Conventions

- All exported symbols must have doc comments.
- Tests live alongside the code they test (`_test.go`).
- Keep the public API minimal; prefer `internal/` for implementation details.
