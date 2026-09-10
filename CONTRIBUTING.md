# Contributing

Argos follows a strict build order: the core module must be complete, tested, and
benchmarked before any integration module is started, and each later phase builds only
on what the previous phase shipped. Do not start work on a later phase's module until
the current phase's exit criteria are green in CI.

## Requirements for every PR

- Tests pass for every module touched (`make test`).
- Benchmarks included for any hot-path code, with `testing.AllocsPerRun` documenting
  allocations per operation (`make bench`). A PR that regresses allocations/throughput
  without justification will be blocked.
- `make fmt` applied (Go via `golangci-lint fmt`, Markdown/YAML via Prettier, `.proto`
  via `buf format`) - CI enforces this (`lint.yml` for Go, `format-check.yml` for the
  rest) and fails on unformatted files rather than fixing them for you.
- `make lint` and `make sec-scan` clean (no new high/critical findings).
- New integrations implement the `Instrumentable` interface and never modify
  `core/` internals directly.

## Workspace

This repo is a `go.work` multi-module workspace. Add new modules with
`go work use ./path/to/module`; never add `replace` directives to a module's own
`go.mod` for in-repo dependencies - `go.work` handles that for local development, and
consumers outside this repo resolve tagged versions normally.
