# SFTP

`integrations/sftp` wraps `github.com/pkg/sftp.Client` by embedding it: since
it's a concrete struct (not an interface) with no `context.Context`
parameter, the wrapper adds `*Context`-suffixed sibling methods
(`OpenContext`, `CreateContext`, ...) for the handful of operations worth a
span, while every other method - `Close`, `Chmod`, `Getwd`, ... - keeps
working unmodified via Go's method promotion.

```go
import argossftp "github.com/jhonsferg/argos/integrations/sftp"

rawClient, _ := sftp.NewClient(sshClient)
client := argossftp.Wrap(rawClient)

f, err := client.OpenContext(ctx, "/reports/latest.csv")
```

## Options

- `WithPathAttribute(bool)` - records `sftp.path` on the span. Off by
  default (paths can carry sensitive filenames).
- `WithLogger(logging.Logger)`.

## See it running

[`reports-service`](https://github.com/jhonsferg/argos/tree/main/samples/reports-service)
pulls a report file over SFTP and records its metadata in Cassandra.
