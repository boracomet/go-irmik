# Database & migrations (Phase 2.2)

Irmik’s data stack is intentionally thin: `database/sql` plus versioned SQL files. ORM (GORM) is optional.

## Packages

| Package | Role |
|---------|------|
| [`irmik/db`](../irmik/db) | Open postgres / sqlite / mysql; thin `Database` wrapper (**no drivers linked by default**) |
| [`irmik/db/sqlite`](../irmik/db/sqlite) | Blank-import: registers modernc SQLite |
| [`irmik/db/postgres`](../irmik/db/postgres) | Blank-import: registers pgx |
| [`irmik/db/mysql`](../irmik/db/mysql) | Blank-import: registers MySQL |
| [`irmik/db/gormx`](../irmik/db/gormx) | Optional GORM open helper on an existing `*db.Database` |
| [`irmik/migrate`](../irmik/migrate) | golang-migrate wrapper: `Up`, `Down`, `Steps`, `Status`, `Create` |

## Config

`irmik.yaml`:

```yaml
database:
  driver: sqlite          # postgres | pgx | sqlite | mysql
  dsn: ./data/app.db      # or url: (DATABASE_URL)
  maxOpenConns: 10
  maxIdleConns: 5
  migratePath: migrations
```

Environment overrides:

| Variable | Field |
|----------|--------|
| `DATABASE_URL` / `IRMIK_DATABASE_URL` | `database.url` |
| `IRMIK_DB_DSN` | `database.dsn` |
| `IRMIK_DB_DRIVER` | `database.driver` |
| `IRMIK_MIGRATE_PATH` | `database.migratePath` |
| `IRMIK_DB_MAX_OPEN` / `IRMIK_DB_MAX_IDLE` | pool sizes |

DSN wins over URL when both are set.

## Open a database

Drivers are **opt-in** so unused SQL drivers are not linked into your binary:

```go
import (
    "github.com/boracomet/go-irmik/irmik/db"
    _ "github.com/boracomet/go-irmik/irmik/db/sqlite" // or postgres / mysql
)

cfg, _ := config.Load("irmik.yaml")
database, err := db.OpenFromConfig(cfg.Database)
if err != nil {
    return err
}
defer database.Close()

_ = database.Ping(ctx)
sqlDB := database.DB() // *sql.DB
```

Or without config:

```go
database, err := db.Open(db.Options{
    Driver: "postgres",
    DSN:    "postgres://user:pass@localhost:5432/app?sslmode=disable",
})
```

(Requires `_ "github.com/boracomet/go-irmik/irmik/db/postgres"`.)

### Drivers

| Config name | Blank-import package | `database/sql` name | Notes |
|-------------|----------------------|---------------------|--------|
| `postgres`, `postgresql`, `pgx` | `irmik/db/postgres` | `pgx` | Preferred Postgres path |
| `sqlite`, `sqlite3` | `irmik/db/sqlite` | `sqlite` | Pure Go; no CGO |
| `mysql`, `mariadb` | `irmik/db/mysql` | `mysql` | Optional / nice-to-have |

Opening without the matching blank-import returns a clear error naming the package to import.

The `irmik` CLI blank-imports all three so `irmik migrate` works for any configured driver.

## Migrations

Place numbered SQL pairs under `database.migratePath` (default `migrations/`):

```text
migrations/
  000001_users.up.sql
  000001_users.down.sql
  20260810120000_posts.up.sql
  20260810120000_posts.down.sql
```

Compatible with [golang-migrate](https://github.com/golang-migrate/migrate) naming. Version history is stored in `schema_migrations`.

### Library API

```go
m, err := migrate.Open(migrate.Options{
    Driver: database.Driver(),
    DB:     database.DB(),
    Path:   "migrations",
})
if err != nil {
    return err
}
defer m.Close()

if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
    return err
}
st, _ := m.Status()
_ = m.Steps(-1) // roll back one
_ = m.Down()    // all the way down
```

Embed migrations with `Options.FS` + `Options.FSPath` (`io/fs` / `embed.FS`).

### CLI

```bash
irmik migrate create add_users   # writes timestamped up/down files
irmik migrate up                 # apply all pending
irmik migrate up --steps 1       # apply one
irmik migrate down               # roll back all
irmik migrate down --steps 1     # roll back one
irmik migrate status             # version + dirty flag
```

Requires `database.driver` + DSN/URL (except `create`, which only needs `migratePath`).

## Optional GORM

```go
import (
    "github.com/boracomet/go-irmik/irmik/db/gormx"
    _ "github.com/boracomet/go-irmik/irmik/db/sqlite"
)

gdb, err := gormx.Open(database)
```

GORM is not required by the framework core; use it only when you want an ORM.
