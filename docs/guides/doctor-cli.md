# Doctor CLI

`cmd/doctor` scans a Go project's own source (via `go/packages`, not just
`go.mod`) for vendor SDKs it already imports - an HTTP router, a database
driver, a message broker client - and reports any that don't yet have a
matching Argos integration imported alongside them.

```bash
go run github.com/jhonsferg/argos/cmd/doctor [-strict] [path]
```

Example output against a project using gin without Argos wired up yet:

```
Argos Doctor - scanned .

[suggest] Argos core detected - add github.com/jhonsferg/argos
           call argos.Init() once at startup before adding any integration.
[suggest] gin detected - add github.com/jhonsferg/argos/integrations/httpserver/gin
           wrap your gin.Engine with argosgin's middleware.

2 integration(s) suggested.
```

`-strict` exits with status 1 if any integration is suggested, for use as a
CI check - path defaults to `.` if omitted.

## Scaffolding a new service

```bash
go run github.com/jhonsferg/argos/cmd/doctor -init [-force] [path]
```

Writes a starter `argos.config.yaml` (see [Configuration](configuration.md))
for the target project - `service_name` seeded from its `go.mod` module
path, an `integrations:` section pre-filled if `database/sql` is already in
use - and prints a ready-to-paste `main.go` snippet built around
[`argos.Run`](../quickstart.md), followed by the same suggestions the
normal scan would print. Refuses to overwrite an existing
`argos.config.yaml` unless `-force` is also passed. It never touches
`main.go` itself - only the config file is written; the snippet is printed
for you to paste and adjust.

## How it decides

`CollectImports` loads the project's own packages (including tests) and
collects the import paths appearing directly in its source - deliberately
not following imports transitively past that first level, since a
dependency-of-a-dependency happening to import, say, Kafka's client
internally isn't evidence the project itself uses Kafka. `Suggest` then
evaluates a pure rule table (`Rule{Name, Vendor, ArgosModule, Hint}`) against
that set - see `cmd/doctor/rules.go` for the full list.
