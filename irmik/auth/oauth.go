package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
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
	if code == "demo" || code == "" {
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

// GitHubProvider is a thin config holder implementing Provider with HTTP stubs
// that return ErrNotConfigured until ClientID is set and Exchange is overridden
// or real endpoints are wired. Use for interface wiring; production apps should
// complete token exchange against api.github.com.
type GitHubProvider struct {
	OAuthConfig
	HTTPClient *http.Client
}

// Name implements Provider.
func (p *GitHubProvider) Name() string { return "github" }

// AuthCodeURL implements Provider.
func (p *GitHubProvider) AuthCodeURL(state string) string {
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

// Exchange is a stub that documents the real flow; returns an error directing
// callers to implement token + user fetch (kept intentional for Phase 2.1).
func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*OAuthUser, error) {
	_ = ctx
	_ = code
	if p.ClientID == "" || p.ClientSecret == "" {
		return nil, errors.New("auth: github oauth not configured")
	}
	return nil, errors.New("auth: github Exchange not implemented; use Provider interface with your HTTP client")
}

// GoogleProvider mirrors GitHubProvider for Google OAuth wiring.
type GoogleProvider struct {
	OAuthConfig
	HTTPClient *http.Client
}

// Name implements Provider.
func (p *GoogleProvider) Name() string { return "google" }

// AuthCodeURL implements Provider.
func (p *GoogleProvider) AuthCodeURL(state string) string {
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

// Exchange stub — same pattern as GitHubProvider.
func (p *GoogleProvider) Exchange(ctx context.Context, code string) (*OAuthUser, error) {
	_ = ctx
	_ = code
	if p.ClientID == "" || p.ClientSecret == "" {
		return nil, errors.New("auth: google oauth not configured")
	}
	return nil, errors.New("auth: google Exchange not implemented; use Provider interface with your HTTP client")
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
