package auth_test

import (
	"testing"
	"time"

	"github.com/boracomet/go-irmik/irmik/auth"
)

func TestPasswordArgon2AndBcrypt(t *testing.T) {
	hash, err := auth.HashPassword("s3cret!!", auth.PasswordOptions{Algo: auth.AlgoArgon2id})
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.CheckPassword(hash, "s3cret!!"); err != nil {
		t.Fatal(err)
	}
	if err := auth.CheckPassword(hash, "wrong"); err == nil {
		t.Fatal("expected failure")
	}

	bhash, err := auth.HashPassword("s3cret!!", auth.PasswordOptions{Algo: auth.AlgoBcrypt})
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.CheckPassword(bhash, "s3cret!!"); err != nil {
		t.Fatal(err)
	}
}

func TestJWTIssueParse(t *testing.T) {
	a := auth.New(auth.Config{
		JWTSecret: "test-secret-at-least-32-chars-long!!",
		JWTIssuer: "irmik-test",
		AccessTTL: time.Minute,
	})
	tok, exp, err := a.IssueAccessToken(auth.User{
		ID:    "u1",
		Email: "u@example.com",
		Roles: []string{"admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" || exp.Before(time.Now()) {
		t.Fatalf("bad token/exp")
	}
	claims, err := a.ParseAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "u1" || claims.Email != "u@example.com" {
		t.Fatalf("%+v", claims)
	}
	if _, err := a.ParseAccessToken(tok + "x"); err != auth.ErrInvalidToken {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestStubOAuth(t *testing.T) {
	p := &auth.StubProvider{ProviderName: "github-stub"}
	url := p.AuthCodeURL("xyz")
	if url == "" {
		t.Fatal("empty url")
	}
	u, err := p.Exchange(t.Context(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email == "" || u.Provider != "github-stub" {
		t.Fatalf("%+v", u)
	}
}
