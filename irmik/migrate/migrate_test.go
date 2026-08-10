package migrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boracomet/go-irmik/irmik/db"
	"github.com/boracomet/go-irmik/irmik/migrate"
)

func writeMigrations(t *testing.T, dir string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "000001_users.up.sql"), `CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);`)
	mustWrite(t, filepath.Join(dir, "000001_users.down.sql"), `DROP TABLE IF EXISTS users;`)
	mustWrite(t, filepath.Join(dir, "000002_posts.up.sql"), `CREATE TABLE posts (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL
);`)
	mustWrite(t, filepath.Join(dir, "000002_posts.down.sql"), `DROP TABLE IF EXISTS posts;`)
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestUpDownStatusSteps(t *testing.T) {
	dir := t.TempDir()
	writeMigrations(t, dir)

	dbPath := filepath.Join(t.TempDir(), "migrate.db")
	database, err := db.Open(db.Options{Driver: "sqlite", DSN: dbPath})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}

	m, err := migrate.Open(migrate.Options{
		Driver: database.Driver(),
		DB:     database.DB(),
		Path:   dir,
	})
	if err != nil {
		_ = database.Close()
		t.Fatalf("migrate.Open: %v", err)
	}
	// Migrator.Close closes the *sql.DB pool.
	defer m.Close()

	st, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Empty {
		t.Fatalf("expected empty status before migrations, got %+v", st)
	}

	if err := m.Up(); err != nil {
		t.Fatalf("Up: %v", err)
	}
	st, err = m.Status()
	if err != nil {
		t.Fatalf("Status after Up: %v", err)
	}
	if st.Empty || st.Version != 2 || st.Dirty {
		t.Fatalf("status after Up = %+v, want version=2 clean", st)
	}

	if err := m.Up(); err != migrate.ErrNoChange {
		t.Fatalf("second Up = %v, want ErrNoChange", err)
	}

	var n int
	if err := database.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('users','posts')`).Scan(&n); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if n != 2 {
		t.Fatalf("tables = %d, want 2", n)
	}

	if err := m.Steps(-1); err != nil {
		t.Fatalf("Steps(-1): %v", err)
	}
	st, err = m.Status()
	if err != nil {
		t.Fatalf("Status after Steps: %v", err)
	}
	if st.Version != 1 {
		t.Fatalf("version after Steps(-1) = %d, want 1", st.Version)
	}

	if err := m.Down(); err != nil {
		t.Fatalf("Down: %v", err)
	}
	st, err = m.Status()
	if err != nil {
		t.Fatalf("Status after Down: %v", err)
	}
	if !st.Empty {
		t.Fatalf("expected empty after Down, got %+v", st)
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	up, down, err := migrate.Create(dir, "Add Users")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if filepath.Ext(up) != ".sql" || filepath.Ext(down) != ".sql" {
		t.Fatalf("unexpected paths: %s %s", up, down)
	}
	if _, err := os.Stat(up); err != nil {
		t.Fatalf("up missing: %v", err)
	}
	if _, err := os.Stat(down); err != nil {
		t.Fatalf("down missing: %v", err)
	}
}
