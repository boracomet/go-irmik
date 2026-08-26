package irmik

import (
	"testing"

	"github.com/boracomet/go-irmik/irmik/config"
)

func TestNewRejectsWeakJWTSecretOutsideDevelopment(t *testing.T) {
	for _, secret := range []string{"", "dev-only-change-me-jwt-secret-32b", "not-long-enough"} {
		cfg := config.Default()
		cfg.App.Env = "production"
		cfg.Auth.JWTSecret = secret
		if _, err := New(cfg); err == nil {
			t.Fatalf("New accepted production secret %q", secret)
		}
	}
}

func TestNewAllowsDevelopmentWithoutJWTSecret(t *testing.T) {
	cfg := config.Default()
	app, err := New(cfg)
	if err != nil {
		t.Fatalf("New development app: %v", err)
	}
	if app.Devtools == nil {
		t.Fatal("expected Devtools in development")
	}
}

func TestNewProductionHasNoDevtools(t *testing.T) {
	cfg := config.Default()
	cfg.App.Env = "production"
	cfg.Auth.JWTSecret = "production-jwt-secret-value-32chars"
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if app.Devtools != nil {
		t.Fatal("devtools must not mount in production")
	}
}
