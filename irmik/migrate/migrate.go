// Package migrate runs versioned SQL migrations using golang-migrate.
package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "github.com/golang-migrate/migrate/v4/source/file" // file:// migration source
)

// Re-export sentinel errors for callers.
var (
	ErrNoChange = migrate.ErrNoChange
	ErrNilDB    = errors.New("migrate: nil database")
)

// Migrator applies up/down SQL migrations tracked in a schema_migrations table.
type Migrator struct {
	m *migrate.Migrate
}

// Options configures a Migrator.
type Options struct {
	// Driver is the normalized database driver: postgres | sqlite | mysql.
	Driver string
	// DB is an open *sql.DB (same pool the app uses).
	DB *sql.DB
	// Path is a filesystem directory of *.up.sql / *.down.sql files.
	// Ignored when FS is set.
	Path string
	// FS is an optional io/fs source (e.g. embed.FS). When set, FSPath is the
	// subdirectory inside FS (default ".").
	FS     fs.FS
	FSPath string
	// MigrationsTable overrides the version table name (default schema_migrations).
	MigrationsTable string
}

// Status describes the current migration version.
type Status struct {
	Version uint
	Dirty   bool
	// Empty is true when no migrations have been applied yet.
	Empty bool
}

// Open builds a Migrator from Options.
func Open(opts Options) (*Migrator, error) {
	if opts.DB == nil {
		return nil, ErrNilDB
	}
	driverName := strings.ToLower(strings.TrimSpace(opts.Driver))
	dbDriver, err := databaseDriver(driverName, opts.DB, opts.MigrationsTable)
	if err != nil {
		return nil, err
	}

	var (
		m   *migrate.Migrate
		src source.Driver
	)

	if opts.FS != nil {
		sub := opts.FSPath
		if sub == "" {
			sub = "."
		}
		src, err = iofs.New(opts.FS, sub)
		if err != nil {
			return nil, fmt.Errorf("migrate: iofs source: %w", err)
		}
		m, err = migrate.NewWithInstance("iofs", src, driverName, dbDriver)
	} else {
		path := strings.TrimSpace(opts.Path)
		if path == "" {
			return nil, fmt.Errorf("migrate: empty migrations path")
		}
		var abs string
		abs, err = filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("migrate: abs path: %w", err)
		}
		if st, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("migrate: migrations path %q: %w", abs, err)
		} else if !st.IsDir() {
			return nil, fmt.Errorf("migrate: migrations path %q is not a directory", abs)
		}
		sourceURL := "file://" + filepath.ToSlash(abs)
		m, err = migrate.NewWithDatabaseInstance(sourceURL, driverName, dbDriver)
	}
	if err != nil {
		return nil, fmt.Errorf("migrate: open: %w", err)
	}
	return &Migrator{m: m}, nil
}

func databaseDriver(driver string, db *sql.DB, table string) (database.Driver, error) {
	switch driver {
	case "postgres", "postgresql", "pgx":
		cfg := &postgres.Config{}
		if table != "" {
			cfg.MigrationsTable = table
		}
		return postgres.WithInstance(db, cfg)
	case "sqlite", "sqlite3":
		cfg := &sqlite.Config{}
		if table != "" {
			cfg.MigrationsTable = table
		}
		return sqlite.WithInstance(db, cfg)
	case "mysql", "mariadb":
		cfg := &mysql.Config{}
		if table != "" {
			cfg.MigrationsTable = table
		}
		return mysql.WithInstance(db, cfg)
	default:
		return nil, fmt.Errorf("migrate: unsupported driver %q", driver)
	}
}

// Up applies all pending up migrations.
func (m *Migrator) Up() error {
	if m == nil || m.m == nil {
		return ErrNilDB
	}
	err := m.m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return ErrNoChange
	}
	return err
}

// Down rolls back all applied migrations.
func (m *Migrator) Down() error {
	if m == nil || m.m == nil {
		return ErrNilDB
	}
	err := m.m.Down()
	if errors.Is(err, migrate.ErrNoChange) {
		return ErrNoChange
	}
	return err
}

// Steps applies n migrations (positive = up, negative = down).
func (m *Migrator) Steps(n int) error {
	if m == nil || m.m == nil {
		return ErrNilDB
	}
	err := m.m.Steps(n)
	if errors.Is(err, migrate.ErrNoChange) {
		return ErrNoChange
	}
	return err
}

// Status returns the current schema version.
func (m *Migrator) Status() (Status, error) {
	if m == nil || m.m == nil {
		return Status{}, ErrNilDB
	}
	v, dirty, err := m.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return Status{Empty: true}, nil
	}
	if err != nil {
		return Status{}, err
	}
	return Status{Version: v, Dirty: dirty}, nil
}

// Close releases migrate source resources and closes the underlying *sql.DB
// (golang-migrate database drivers close the pool on Close). Do not use the
// shared *sql.DB after calling Close.
func (m *Migrator) Close() error {
	if m == nil || m.m == nil {
		return nil
	}
	srcErr, dbErr := m.m.Close()
	if srcErr != nil {
		return srcErr
	}
	return dbErr
}

var nameSafe = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// Create writes empty up/down migration files under dir.
// Returns the created paths (up, down).
func Create(dir, name string) (upPath, downPath string, err error) {
	if strings.TrimSpace(dir) == "" {
		return "", "", fmt.Errorf("migrate: empty migrations directory")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", fmt.Errorf("migrate: empty migration name")
	}
	slug := strings.ToLower(nameSafe.ReplaceAllString(name, "_"))
	slug = strings.Trim(slug, "_")
	if slug == "" {
		return "", "", fmt.Errorf("migrate: invalid migration name %q", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	version := time.Now().UTC().Format("20060102150405")
	upPath = filepath.Join(dir, fmt.Sprintf("%s_%s.up.sql", version, slug))
	downPath = filepath.Join(dir, fmt.Sprintf("%s_%s.down.sql", version, slug))
	upBody := fmt.Sprintf("-- %s_%s (up)\n", version, slug)
	downBody := fmt.Sprintf("-- %s_%s (down)\n", version, slug)
	if err := os.WriteFile(upPath, []byte(upBody), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(downPath, []byte(downBody), 0o644); err != nil {
		return "", "", err
	}
	return upPath, downPath, nil
}
