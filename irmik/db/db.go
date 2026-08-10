// Package db opens and wraps database/sql connections for Irmik apps.
//
// Drivers are not linked by default. Blank-import the driver you need:
//
//	import _ "github.com/boracomet/go-irmik/irmik/db/sqlite"
//	import _ "github.com/boracomet/go-irmik/irmik/db/postgres"
//	import _ "github.com/boracomet/go-irmik/irmik/db/mysql"
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/boracomet/go-irmik/irmik/config"
)

// Database is a thin wrapper around *sql.DB with driver metadata.
type Database struct {
	sql    *sql.DB
	driver string // normalized: postgres | sqlite | mysql
}

// Options configures Open.
type Options struct {
	Driver       string
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
}

// OpenFromConfig opens a database using config.DatabaseConfig.
func OpenFromConfig(cfg config.DatabaseConfig) (*Database, error) {
	return Open(Options{
		Driver:       cfg.DriverName(),
		DSN:          cfg.DSNOrURL(),
		MaxOpenConns: cfg.MaxOpenConns,
		MaxIdleConns: cfg.MaxIdleConns,
	})
}

// Open opens a *sql.DB for the given driver and DSN.
// Supported drivers: postgres/pgx, sqlite, mysql (each via blank-import packages).
func Open(opts Options) (*Database, error) {
	driver, sqlDriver, err := resolveDriver(opts.Driver)
	if err != nil {
		return nil, err
	}
	if !driverRegistered(sqlDriver) {
		return nil, fmt.Errorf("db: driver %q not registered; blank-import github.com/boracomet/go-irmik/irmik/db/%s", sqlDriver, importHint(driver))
	}
	dsn := strings.TrimSpace(opts.DSN)
	if dsn == "" {
		return nil, fmt.Errorf("db: empty DSN for driver %q", driver)
	}
	if driver == "sqlite" {
		dsn = normalizeSQLiteDSN(dsn)
	}

	sqlDB, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", driver, err)
	}

	if opts.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.ConnMaxLife > 0 {
		sqlDB.SetConnMaxLifetime(opts.ConnMaxLife)
	}

	db := &Database{sql: sqlDB, driver: driver}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Ping(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: ping %s: %w", driver, err)
	}
	return db, nil
}

// Driver returns the normalized driver name (postgres|sqlite|mysql).
func (d *Database) Driver() string {
	if d == nil {
		return ""
	}
	return d.driver
}

// DB returns the underlying *sql.DB.
func (d *Database) DB() *sql.DB {
	if d == nil {
		return nil
	}
	return d.sql
}

// Ping verifies the connection.
func (d *Database) Ping(ctx context.Context) error {
	if d == nil || d.sql == nil {
		return fmt.Errorf("db: nil database")
	}
	return d.sql.PingContext(ctx)
}

// Close closes the underlying connection pool.
func (d *Database) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

// resolveDriver maps config names to (normalized, database/sql driver name).
func resolveDriver(name string) (normalized, sqlName string, err error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "postgres", "postgresql", "pgx":
		return "postgres", "pgx", nil
	case "sqlite", "sqlite3":
		return "sqlite", "sqlite", nil
	case "mysql", "mariadb":
		return "mysql", "mysql", nil
	default:
		return "", "", fmt.Errorf("db: unsupported driver %q (want postgres, sqlite, or mysql)", name)
	}
}

func driverRegistered(sqlName string) bool {
	for _, d := range sql.Drivers() {
		if d == sqlName {
			return true
		}
	}
	return false
}

func importHint(normalized string) string {
	switch normalized {
	case "postgres":
		return "postgres"
	case "mysql":
		return "mysql"
	default:
		return "sqlite"
	}
}

// normalizeSQLiteDSN accepts bare paths and :memory: forms for modernc.
func normalizeSQLiteDSN(dsn string) string {
	switch {
	case dsn == ":memory:" || strings.HasPrefix(dsn, "file:"):
		return dsn
	case strings.HasPrefix(dsn, "sqlite://"):
		return strings.TrimPrefix(dsn, "sqlite://")
	default:
		return dsn
	}
}
