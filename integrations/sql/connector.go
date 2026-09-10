package argossql

import (
	"context"
	"database/sql"
	"database/sql/driver"
)

// WrapConnector returns a driver.Connector that instruments every connection
// it produces, for drivers exposed via sql.OpenDB rather than a DSN string.
func WrapConnector(c driver.Connector, opts ...Option) driver.Connector {
	return &wrappedConnector{connector: c, cfg: newConfig(opts...)}
}

type wrappedConnector struct {
	connector driver.Connector
	cfg       *config
}

func (c *wrappedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &wrappedConn{conn: conn, cfg: c.cfg}, nil
}

func (c *wrappedConnector) Driver() driver.Driver {
	return Wrap(c.connector.Driver(), withConfig(c.cfg))
}

// dsnConnector replicates database/sql's own unexported dsnConnector: the
// shim it uses internally so sql.Open(name, dsn) can be expressed as
// sql.OpenDB(connector) for drivers that only implement driver.Open(dsn),
// not driver.DriverContext. We need our own copy since the stdlib's isn't
// exported.
type dsnConnector struct {
	dsn    string
	driver driver.Driver
}

func (t dsnConnector) Connect(context.Context) (driver.Conn, error) {
	return t.driver.Open(t.dsn)
}

func (t dsnConnector) Driver() driver.Driver {
	return t.driver
}

// Open wraps the driver registered under driverName (via sql.Register - the
// same registration any database/sql driver package does in its own init)
// and opens *sql.DB through the wrapped driver. driverName must already be
// registered, typically by blank-importing the driver package.
func Open(driverName, dsn string, opts ...Option) (*sql.DB, error) {
	probe, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	underlying := probe.Driver()
	_ = probe.Close() // sql.Open doesn't connect eagerly; nothing to release.

	wrapped := Wrap(underlying, opts...)
	if dc, ok := wrapped.(driver.DriverContext); ok {
		connector, err := dc.OpenConnector(dsn)
		if err != nil {
			return nil, err
		}
		return sql.OpenDB(connector), nil
	}
	return sql.OpenDB(dsnConnector{dsn: dsn, driver: wrapped}), nil
}
