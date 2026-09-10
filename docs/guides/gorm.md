# GORM

`integrations/gorm` instruments GORM via its `Plugin` interface, hooking the
same before/after callback pairs `gorm.io/plugin/opentelemetry` uses
(Create/Query/Update/Delete/Row/Raw). It's a peer of [SQL](sql.md), not a
replacement: this adds ORM-level spans (operation, table); opening the
underlying `*sql.DB` through `argossql.Open` additionally instruments the
actual driver round trip, so a single query produces spans from both layers.

```go
import (
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	argosgorm "github.com/jhonsferg/argos/integrations/gorm"
	argossql "github.com/jhonsferg/argos/integrations/sql"
)

sqlDB, _ := argossql.Open("pgx", dsn, argossql.WithSystem(semconv.DBSystemPostgreSQL))
db, _ := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{})
db.Use(argosgorm.New(argosgorm.WithSystem(semconv.DBSystemPostgreSQL)))
```

## Options

Same shape as [SQL](sql.md): `WithSystem`, `WithQueryText` (off by default),
`WithMaskedQueryText` (records the SQL with literals redacted, e.g.
`WHERE name=?` - masked wins if both are enabled), `WithLogger`,
`WithYAMLConfig` (`query_text`/`query_text_masked` under `integrations.gorm`).

Note that GORM's own query builder already parameterizes most statements it
generates (`Create`/`Find`/... produce `?`-placeholder SQL with no literal
values embedded), so masking mainly matters for hand-written `db.Raw(...)`
queries.

## See it running

[`catalog-service`](https://github.com/jhonsferg/argos/tree/main/samples/catalog-service)
persists via GORM-over-argossql and publishes a Kafka event on create.
