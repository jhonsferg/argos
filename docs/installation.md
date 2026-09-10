# Installation

Argos is a Go workspace of independent modules. Install `core` plus whichever
integrations you actually use - nothing else is pulled in.

```bash
go get github.com/jhonsferg/argos
```

Then add integrations one at a time, e.g.:

```bash
go get github.com/jhonsferg/argos/integrations/httpserver/gin
go get github.com/jhonsferg/argos/integrations/redis
go get github.com/jhonsferg/argos/integrations/kafka
```

Requires Go 1.26 or later - earlier versions are not supported.

## Which integration do I need?

Run [`cmd/doctor`](guides/doctor-cli.md) against your own project - it scans
your imports and tells you:

```bash
go run github.com/jhonsferg/argos/cmd/doctor
```

Or browse the [Guides](guides/http-servers.md) section for the exact
package name and wiring snippet per technology.
