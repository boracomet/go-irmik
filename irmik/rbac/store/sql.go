package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/boracomet/go-irmik/irmik/rbac"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// SQL is a database/sql-backed Store. Blank-import your driver (e.g. irmik/db/sqlite).
// Works with SQLite and Postgres (placeholders rebound for postgres/pgx).
type SQL struct {
	db     *sql.DB
	driver string
}

// NewSQL wraps db. driver is "sqlite", "postgres", "pgx", or "mysql" (affects placeholders).
func NewSQL(db *sql.DB, driver string) *SQL {
	return &SQL{db: db, driver: strings.ToLower(strings.TrimSpace(driver))}
}

// EnsureSchema applies embedded migrations/rbac DDL (CREATE TABLE IF NOT EXISTS).
func (s *SQL) EnsureSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("rbac/store: nil sql store")
	}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("rbac/store: apply %s: %w", name, err)
		}
	}
	return nil
}

// RolePermissions implements Store.
func (s *SQL) RolePermissions(ctx context.Context) (map[string][]string, error) {
	q := s.rebind(`SELECT role_name, permission_name FROM role_permissions ORDER BY role_name, permission_name`)
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]string)
	for rows.Next() {
		var role, perm string
		if err := rows.Scan(&role, &perm); err != nil {
			return nil, err
		}
		out[role] = append(out[role], perm)
	}
	return out, rows.Err()
}

// UserRoles implements Store.
func (s *SQL) UserRoles(ctx context.Context) (map[string][]string, error) {
	q := s.rebind(`SELECT user_id, role_name FROM user_roles ORDER BY user_id, role_name`)
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]string)
	for rows.Next() {
		var userID, role string
		if err := rows.Scan(&userID, &role); err != nil {
			return nil, err
		}
		out[userID] = append(out[userID], role)
	}
	return out, rows.Err()
}

// UpsertRole implements Store.
func (s *SQL) UpsertRole(ctx context.Context, name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	q := s.rebind(`INSERT INTO roles (name, description) VALUES (?, ?)
ON CONFLICT (name) DO UPDATE SET description = CASE
  WHEN excluded.description = '' THEN roles.description ELSE excluded.description END`)
	_, err := s.db.ExecContext(ctx, q, name, description)
	return err
}

// UpsertPermission implements Store.
func (s *SQL) UpsertPermission(ctx context.Context, name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	q := s.rebind(`INSERT INTO permissions (name, description) VALUES (?, ?)
ON CONFLICT (name) DO UPDATE SET description = CASE
  WHEN excluded.description = '' THEN permissions.description ELSE excluded.description END`)
	_, err := s.db.ExecContext(ctx, q, name, description)
	return err
}

// Grant implements Store.
func (s *SQL) Grant(ctx context.Context, role, permission string) error {
	role = strings.TrimSpace(role)
	permission = strings.TrimSpace(permission)
	if role == "" || permission == "" {
		return nil
	}
	if err := s.UpsertRole(ctx, role, ""); err != nil {
		return err
	}
	if err := s.UpsertPermission(ctx, permission, ""); err != nil {
		return err
	}
	q := s.rebind(`INSERT INTO role_permissions (role_name, permission_name) VALUES (?, ?)
ON CONFLICT (role_name, permission_name) DO NOTHING`)
	_, err := s.db.ExecContext(ctx, q, role, permission)
	return err
}

// AssignRole implements Store.
func (s *SQL) AssignRole(ctx context.Context, userID, role string) error {
	userID = strings.TrimSpace(userID)
	role = strings.TrimSpace(role)
	if userID == "" || role == "" {
		return nil
	}
	if err := s.UpsertRole(ctx, role, ""); err != nil {
		return err
	}
	q := s.rebind(`INSERT INTO user_roles (user_id, role_name) VALUES (?, ?)
ON CONFLICT (user_id, role_name) DO NOTHING`)
	_, err := s.db.ExecContext(ctx, q, userID, role)
	return err
}

// LoadRegistry is a convenience wrapper around store.LoadRegistry.
func (s *SQL) LoadRegistry(ctx context.Context) (*rbac.Registry, error) {
	return LoadRegistry(ctx, s)
}

func (s *SQL) rebind(q string) string {
	switch s.driver {
	case "postgres", "pgx", "postgresql":
		return rebindDollar(q)
	default:
		return q
	}
}

func rebindDollar(q string) string {
	var b strings.Builder
	b.Grow(len(q) + 8)
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(fmt.Sprintf("%d", n))
			continue
		}
		b.WriteByte(q[i])
	}
	return b.String()
}

// MigrationsFS exposes the embedded SQL for apps that prefer irmik/migrate.
func MigrationsFS() embed.FS { return migrationsFS }
