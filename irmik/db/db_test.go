package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/db"
)

func TestOpenSQLiteTempFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(db.Options{
		Driver: "sqlite",
		DSN:    path,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if got := database.Driver(); got != "sqlite" {
		t.Fatalf("Driver = %q, want sqlite", got)
	}
	if database.DB() == nil {
		t.Fatal("DB() is nil")
	}
	if err := database.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestOpenFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.db")
	cfg := config.DatabaseConfig{
		Driver:       "sqlite",
		DSN:          path,
		MaxOpenConns: 2,
		MaxIdleConns: 1,
	}
	database, err := db.OpenFromConfig(cfg)
	if err != nil {
		t.Fatalf("OpenFromConfig: %v", err)
	}
	defer database.Close()
}

func TestOpenRejectsEmptyDSN(t *testing.T) {
	_, err := db.Open(db.Options{Driver: "sqlite", DSN: ""})
	if err == nil {
		t.Fatal("expected error for empty DSN")
	}
}

func TestOpenRejectsUnknownDriver(t *testing.T) {
	_, err := db.Open(db.Options{Driver: "oracle", DSN: "x"})
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
}
