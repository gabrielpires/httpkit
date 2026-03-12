# Contributing

Thank you for your interest in contributing to httpkit!

## Dev Setup

1. Install Go 1.25 or later.
2. Fork and clone the repository.
3. Install tooling:
   ```bash
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

## Workflow

1. Create a feature branch from `main`.
2. Make your changes, including tests.
3. Run the full check suite:
   ```bash
   make fmt lint test
   ```
4. Open a pull request against `main`.

## PR Requirements

- All CI checks must pass.
- New behaviour must be covered by tests.
- Public API additions need doc comments.
- Update `CHANGELOG.md` under `## [Unreleased]`.

## Code of Conduct

Please read and follow the [Code of Conduct](CODE_OF_CONDUCT.md).
