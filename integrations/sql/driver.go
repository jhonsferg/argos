package argossql

import "database/sql/driver"

// Wrap returns a driver.Driver that instruments every connection opened
// through d. If d implements driver.DriverContext, the returned driver does
// too, so callers using database/sql's modern connection path keep using it.
func Wrap(d driver.Driver, opts ...Option) driver.Driver {
	wd := &wrappedDriver{driver: d, cfg: newConfig(opts...)}
	if _, ok := d.(driver.DriverContext); ok {
		return &wrappedDriverContext{wrappedDriver: wd}
	}
	return wd
}

type wrappedDriver struct {
	driver driver.Driver
	cfg    *config
}

func (d *wrappedDriver) Open(dsn string) (driver.Conn, error) {
	conn, err := d.driver.Open(dsn)
	if err != nil {
		return nil, err
	}
	return &wrappedConn{conn: conn, cfg: d.cfg}, nil
}

// wrappedDriverContext adds OpenConnector on top of wrappedDriver, only
// constructed when the wrapped driver actually implements
// driver.DriverContext - see Wrap.
type wrappedDriverContext struct {
	*wrappedDriver
}

func (d *wrappedDriverContext) OpenConnector(dsn string) (driver.Connector, error) {
	connector, err := d.driver.(driver.DriverContext).OpenConnector(dsn)
	if err != nil {
		return nil, err
	}
	return WrapConnector(connector, withConfig(d.cfg)), nil
}
