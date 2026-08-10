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
	Build    BuildConfig    `yaml:"build"`
	Content  ContentConfig  `yaml:"content"`
	SEO      SEOConfig      `yaml:"seo"`
	Islands  IslandsConfig  `yaml:"islands"`
	I18n     I18nConfig     `yaml:"i18n"`
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
	}
}

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
