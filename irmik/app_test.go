package irmik

import (
	"testing"

	"github.com/boracomet/go-irmik/irmik/config"
)

func TestNewRejectsWeakJWTSecretOutsideDevelopment(t *testing.T) {
	for _, secret := range []string{"", "dev-only-change-me-jwt-secret-32b"} {
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
	if _, err := New(cfg); err != nil {
		t.Fatalf("New development app: %v", err)
	}
}
