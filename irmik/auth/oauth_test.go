package auth

import (
	"context"
	"errors"
	"testing"
)

func TestStubProviderRejectsEmptyCode(t *testing.T) {
	if _, err := (&StubProvider{}).Exchange(context.Background(), ""); err == nil {
		t.Fatal("expected an empty OAuth code to be rejected")
	}
}

func TestOAuthStubsExchangeNotImplemented(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		p    Provider
	}{
		{"github", &GitHubStub{OAuthConfig: OAuthConfig{ClientID: "id", ClientSecret: "secret"}}},
		{"google", &GoogleStub{OAuthConfig: OAuthConfig{ClientID: "id", ClientSecret: "secret"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.p.Exchange(context.Background(), "code")
			if !errors.Is(err, ErrOAuthNotImplemented) {
				t.Fatalf("Exchange: %v", err)
			}
			if tc.p.AuthCodeURL("st") == "" {
				t.Fatal("empty AuthCodeURL")
			}
		})
	}
}
