package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config is the framework runtime configuration.
type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Cache    CacheConfig    `yaml:"cache"`
	Database DatabaseConfig `yaml:"database"`
	Build    BuildConfig    `yaml:"build"`
	Content  ContentConfig  `yaml:"content"`
	SEO      SEOConfig      `yaml:"seo"`
	Islands  IslandsConfig  `yaml:"islands"`
	I18n     I18nConfig     `yaml:"i18n"`
	Session  SessionConfig  `yaml:"session"`
	Auth     AuthConfig     `yaml:"auth"`
	Realtime RealtimeConfig `yaml:"realtime"`
}

type AppConfig struct {
	Name    string `yaml:"name"`
	Env     string `yaml:"env"` // development | production | test
	BaseURL string `yaml:"baseURL"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"readTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout"`
}

type CacheConfig struct {
	Driver string `yaml:"driver"` // memory | disk | redis
	TTL    time.Duration `yaml:"ttl"`
	DiskDir string `yaml:"diskDir"`
	RedisURL string `yaml:"redisURL"`
}

type BuildConfig struct {
	OutDir     string `yaml:"outDir"`
	PublicDir  string `yaml:"publicDir"`
	AppDir     string `yaml:"appDir"`
	Templates  string `yaml:"templates"`
}

type ContentConfig struct {
	Dir string `yaml:"dir"`
}

type SEOConfig struct {
	SiteName        string `yaml:"siteName"`
	DefaultOGImage  string `yaml:"defaultOGImage"`
	TwitterHandle   string `yaml:"twitterHandle"`
	GenerateSitemap bool   `yaml:"generateSitemap"`
}

type IslandsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Dir       string `yaml:"dir"`
	OutDir    string `yaml:"outDir"`
	DevServer string `yaml:"devServer"` // e.g. http://localhost:5173
}

type I18nConfig struct {
	DefaultLocale string   `yaml:"defaultLocale"`
	Locales       []string `yaml:"locales"`
}

// SessionConfig configures cookie sessions (Phase 2.1).
type SessionConfig struct {
	Driver   string        `yaml:"driver"` // memory | redis
	Name     string        `yaml:"name"`
	Secret   string        `yaml:"secret"`
	MaxAge   time.Duration `yaml:"maxAge"`
	Path     string        `yaml:"path"`
	Domain   string        `yaml:"domain"`
	Secure   *bool         `yaml:"secure"` // nil → !IsDev()
	HTTPOnly *bool         `yaml:"httpOnly"`
	SameSite string        `yaml:"sameSite"` // lax | strict | none
	RedisURL string        `yaml:"redisURL"`
}

// AuthConfig configures JWT and related secrets (Phase 2.1).
type AuthConfig struct {
	JWTSecret string        `yaml:"jwtSecret"`
	JWTIssuer string        `yaml:"jwtIssuer"`
	AccessTTL time.Duration `yaml:"accessTTL"`
}

// RealtimeConfig holds optional WebSocket / SSE settings (Phase 2.3).
type RealtimeConfig struct {
	// AllowedOrigins lists exact Origin values for WebSocket upgrades.
	// Empty allows all origins (fine for local demos; restrict in production).
	AllowedOrigins []string `yaml:"allowedOrigins"`
}

// DatabaseConfig holds SQL connection and migration settings (Phase 2.2).
type DatabaseConfig struct {
	Driver       string `yaml:"driver"` // postgres | pgx | sqlite | mysql
	DSN          string `yaml:"dsn"`
	URL          string `yaml:"url"` // alias for DSN; often set via DATABASE_URL
	MaxOpenConns int    `yaml:"maxOpenConns"`
	MaxIdleConns int    `yaml:"maxIdleConns"`
	MigratePath  string `yaml:"migratePath"`
}

// DSNOrURL returns the connection string (DSN wins over URL when both set).
func (d DatabaseConfig) DSNOrURL() string {
	if strings.TrimSpace(d.DSN) != "" {
		return strings.TrimSpace(d.DSN)
	}
	return strings.TrimSpace(d.URL)
}

// DriverName returns the configured driver, defaulting to postgres when a DSN is set.
func (d DatabaseConfig) DriverName() string {
	name := strings.ToLower(strings.TrimSpace(d.Driver))
	if name != "" {
		return name
	}
	if d.DSNOrURL() != "" {
		return "postgres"
	}
	return ""
}

// Default returns sensible Phase-1 defaults.
func Default() Config {
	return Config{
		App: AppConfig{
			Name:    "irmik",
			Env:     "development",
			BaseURL: "http://localhost:8080",
		},
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Cache: CacheConfig{
			Driver:  "memory",
			TTL:     60 * time.Second,
			DiskDir: ".irmik/cache",
		},
		Build: BuildConfig{
			OutDir:    "out",
			PublicDir: "public",
			AppDir:    "app",
			Templates: "templates",
		},
		Content: ContentConfig{Dir: "content"},
		SEO: SEOConfig{
			SiteName:        "Irmik",
			GenerateSitemap: true,
		},
		Islands: IslandsConfig{
			Enabled:   true,
			Dir:       "islands",
			OutDir:    "public/islands",
			DevServer: "http://localhost:5173",
		},
		I18n: I18nConfig{
			DefaultLocale: "en",
			Locales:       []string{"en"},
		},
		Database: DatabaseConfig{
			Driver:       "",
			MaxOpenConns: 10,
			MaxIdleConns: 5,
			MigratePath:  "migrations",
		},
		Session: SessionConfig{
			Driver:   "memory",
			Name:     "irmik_session",
			MaxAge:   24 * time.Hour,
			Path:     "/",
			SameSite: "lax",
			HTTPOnly: boolPtr(true),
		},
		Auth: AuthConfig{
			JWTIssuer: "irmik",
			AccessTTL: 15 * time.Minute,
		},
		Realtime: RealtimeConfig{},
	}
}

func boolPtr(v bool) *bool { return &v }

// Load merges YAML file (optional), .env (optional), and environment variables.
func Load(path string) (Config, error) {
	cfg := Default()
	_ = godotenv.Load()

	if path == "" {
		path = "irmik.yaml"
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return cfg, err
	}

	applyEnv(&cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("IRMIK_ENV"); v != "" {
		cfg.App.Env = v
	}
	if v := os.Getenv("IRMIK_BASE_URL"); v != "" {
		cfg.App.BaseURL = v
	}
	if v := os.Getenv("IRMIK_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("IRMIK_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("IRMIK_CACHE_DRIVER"); v != "" {
		cfg.Cache.Driver = strings.ToLower(v)
	}
	if v := os.Getenv("REDIS_URL"); v != "" {
		cfg.Cache.RedisURL = v
		if cfg.Session.RedisURL == "" {
			cfg.Session.RedisURL = v
		}
	}
	if v := os.Getenv("IRMIK_SESSION_DRIVER"); v != "" {
		cfg.Session.Driver = strings.ToLower(v)
	}
	if v := os.Getenv("IRMIK_SESSION_SECRET"); v != "" {
		cfg.Session.Secret = v
	}
	if v := os.Getenv("IRMIK_SESSION_NAME"); v != "" {
		cfg.Session.Name = v
	}
	if v := os.Getenv("IRMIK_SESSION_REDIS_URL"); v != "" {
		cfg.Session.RedisURL = v
	}
	if v := os.Getenv("IRMIK_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("IRMIK_AUTH_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("IRMIK_JWT_ISSUER"); v != "" {
		cfg.Auth.JWTIssuer = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("IRMIK_DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("IRMIK_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("IRMIK_DB_DRIVER"); v != "" {
		cfg.Database.Driver = strings.ToLower(v)
	}
	if v := os.Getenv("IRMIK_MIGRATE_PATH"); v != "" {
		cfg.Database.MigratePath = v
	}
	if v := os.Getenv("IRMIK_DB_MAX_OPEN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.MaxOpenConns = n
		}
	}
	if v := os.Getenv("IRMIK_DB_MAX_IDLE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.MaxIdleConns = n
		}
	}
	if v := os.Getenv("IRMIK_WS_ALLOWED_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		origins := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				origins = append(origins, p)
			}
		}
		if len(origins) > 0 {
			cfg.Realtime.AllowedOrigins = origins
		}
	}
}

func (c Config) IsDev() bool {
	return strings.EqualFold(c.App.Env, "development") || c.App.Env == "dev"
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// CacheDriver returns the normalized cache driver name (memory|disk|redis).
func (c Config) CacheDriver() string {
	d := strings.ToLower(strings.TrimSpace(c.Cache.Driver))
	if d == "" {
		return "memory"
	}
	return d
}

// SessionSecure returns whether the session cookie should be Secure.
func (c Config) SessionSecure() bool {
	if c.Session.Secure != nil {
		return *c.Session.Secure
	}
	return !c.IsDev()
}

// SessionHTTPOnly returns whether the session cookie should be HttpOnly.
func (c Config) SessionHTTPOnly() bool {
	if c.Session.HTTPOnly != nil {
		return *c.Session.HTTPOnly
	}
	return true
}
