## What & why

## Checklist

- [ ] Tests added/updated for every module touched (`make test`)
- [ ] Benchmarks added for any hot-path code, with `testing.AllocsPerRun` documenting
      allocations per operation (`make bench`) - and any regression vs `main` is
      justified in this description
- [ ] `make lint` clean
- [ ] `make sec-scan` clean (no new high/critical findings)
- [ ] Docs updated if public API changed
- [ ] If this adds an integration: it implements `Instrumentable` and doesn't modify
      `core/` internals
