package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// Provider is a pluggable OAuth2 provider (Google, GitHub, …).
type Provider interface {
	// Name returns a stable id (e.g. "google", "github").
	Name() string
	// AuthCodeURL returns the provider authorization URL for state.
	AuthCodeURL(state string) string
	// Exchange trades an authorization code for a provider user profile.
	Exchange(ctx context.Context, code string) (*OAuthUser, error)
}

// OAuthUser is the normalized profile returned by a Provider.
type OAuthUser struct {
	Provider  string
	ID        string
	Email     string
	Name      string
	AvatarURL string
	Raw       map[string]any
}

// OAuthConfig holds shared OAuth client settings.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// StubProvider is a workable in-memory OAuth stub for tests and local demos.
// AuthCodeURL encodes the email in the state-less query; Exchange accepts any code
// of the form "user:<id>:<email>" or returns a fixed demo user for code "demo".
type StubProvider struct {
	ProviderName string
	DemoUser     OAuthUser
	BaseAuthURL  string
}

// Name implements Provider.
func (p *StubProvider) Name() string {
	if p.ProviderName == "" {
		return "stub"
	}
	return p.ProviderName
}

// AuthCodeURL implements Provider.
func (p *StubProvider) AuthCodeURL(state string) string {
	base := p.BaseAuthURL
	if base == "" {
		base = "https://example.invalid/oauth/authorize"
	}
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("state", state)
	q.Set("response_type", "code")
	u.RawQuery = q.Encode()
	return u.String()
}

// Exchange implements Provider.
func (p *StubProvider) Exchange(_ context.Context, code string) (*OAuthUser, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("auth: stub oauth code is required")
	}
	if code == "demo" {
		u := p.DemoUser
		if u.ID == "" {
			u = OAuthUser{
				Provider: p.Name(),
				ID:       "stub-1",
				Email:    "demo@example.com",
				Name:     "Demo User",
			}
		}
		if u.Provider == "" {
			u.Provider = p.Name()
		}
		return &u, nil
	}
	return nil, errors.New("auth: stub oauth invalid code")
}

// ErrOAuthNotImplemented is returned by GitHubStub and GoogleStub Exchange.
// Irmik does not ship a GitHub or Google OAuth client.
var ErrOAuthNotImplemented = errors.New("auth: OAuth Exchange is not implemented; use StubProvider for tests or implement Provider with your own HTTP client")

// GitHubStub is not a GitHub OAuth client. AuthCodeURL only builds GitHub's
// authorize URL for wiring tests. Exchange always returns ErrOAuthNotImplemented.
type GitHubStub struct {
	OAuthConfig
}

// Name implements Provider.
func (p *GitHubStub) Name() string { return "github" }

// AuthCodeURL implements Provider (URL builder only).
func (p *GitHubStub) AuthCodeURL(state string) string {
	u, _ := url.Parse("https://github.com/login/oauth/authorize")
	q := u.Query()
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("state", state)
	if len(p.Scopes) > 0 {
		q.Set("scope", joinScopes(p.Scopes))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// Exchange always fails. There is no GitHub token exchange in this package.
func (p *GitHubStub) Exchange(_ context.Context, _ string) (*OAuthUser, error) {
	return nil, ErrOAuthNotImplemented
}

// GoogleStub is not a Google OAuth client. AuthCodeURL only builds Google's
// authorize URL for wiring tests. Exchange always returns ErrOAuthNotImplemented.
type GoogleStub struct {
	OAuthConfig
}

// Name implements Provider.
func (p *GoogleStub) Name() string { return "google" }

// AuthCodeURL implements Provider (URL builder only).
func (p *GoogleStub) AuthCodeURL(state string) string {
	u, _ := url.Parse("https://accounts.google.com/o/oauth2/v2/auth")
	q := u.Query()
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	if len(p.Scopes) > 0 {
		q.Set("scope", joinScopes(p.Scopes))
	} else {
		q.Set("scope", "openid email profile")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// Exchange always fails. There is no Google token exchange in this package.
func (p *GoogleStub) Exchange(_ context.Context, _ string) (*OAuthUser, error) {
	return nil, ErrOAuthNotImplemented
}

func joinScopes(scopes []string) string {
	out := ""
	for i, s := range scopes {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

// Registry maps provider name → Provider.
type Registry map[string]Provider

// Register adds a provider by Name().
func (r Registry) Register(p Provider) {
	r[p.Name()] = p
}

// Get returns a provider by name.
func (r Registry) Get(name string) (Provider, bool) {
	p, ok := r[name]
	return p, ok
}
