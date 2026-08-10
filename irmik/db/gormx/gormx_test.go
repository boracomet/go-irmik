package gormx_test

import (
	"path/filepath"
	"testing"

	"github.com/boracomet/go-irmik/irmik/db"
	"github.com/boracomet/go-irmik/irmik/db/gormx"

	_ "github.com/boracomet/go-irmik/irmik/db/sqlite"
)

func TestOpenSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gorm.db")
	database, err := db.Open(db.Options{Driver: "sqlite", DSN: path})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	gdb, err := gormx.Open(database)
	if err != nil {
		t.Fatalf("gormx.Open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("gdb.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
