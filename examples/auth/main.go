package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik"
	"github.com/boracomet/go-irmik/irmik/auth"
	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/csrf"
	"github.com/boracomet/go-irmik/irmik/middleware"
	"github.com/boracomet/go-irmik/irmik/rbac"
	"github.com/boracomet/go-irmik/irmik/validate"
)

// In-memory demo users: email → {id, password hash, roles}
type demoUser struct {
	ID           string
	Email        string
	PasswordHash string
	Roles        []string
}

func main() {
	cfg, err := config.Load("irmik.yaml")
	if err != nil {
		fatal(err)
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "dev-only-change-me-jwt-secret-32b"
	}
	secure := false
	cfg.Session.Secure = &secure

	app, err := irmik.New(cfg)
	if err != nil {
		fatal(err)
	}
	app.EnableSecureDefaults()
	if err := app.EnableSessions(); err != nil {
		fatal(err)
	}
	authenticator := app.EnableAuth()

	hash, err := auth.HashPasswordDefault("password123")
	if err != nil {
		fatal(err)
	}
	users := map[string]demoUser{
		"admin@example.com": {
			ID: "1", Email: "admin@example.com", PasswordHash: hash, Roles: []string{"admin"},
		},
		"editor@example.com": {
			ID: "2", Email: "editor@example.com", PasswordHash: hash, Roles: []string{"editor"},
		},
	}
	byID := map[string]demoUser{}
	for _, u := range users {
		byID[u.ID] = u
	}

	policies := rbac.New()
	policies.GrantRolePermissions("admin", "dashboard:read", "users:manage")
	policies.GrantRolePermissions("editor", "dashboard:read")

	app.Engine.Use(authenticator.InjectSessionUser(func(userID string) (*auth.User, error) {
		u, ok := byID[userID]
		if !ok {
			return nil, nil
		}
		return &auth.User{ID: u.ID, Email: u.Email, Roles: u.Roles}, nil
	}))

	app.Engine.GET("/", func(c *gin.Context) {
		token := csrf.Ensure(c)
		u, ok := auth.UserFrom(c)
		c.JSON(http.StatusOK, gin.H{
			"app":    "irmik auth demo",
			"authed": ok,
			"user":   u,
			"csrf":   token,
			"hint":   "POST /login with email/password (admin@example.com / password123)",
			"routes": []string{"POST /login", "POST /logout", "GET /me", "GET /admin", "POST /token", "GET /jwt/me"},
		})
	})

	type loginBody struct {
		Email    string `json:"email" form:"email" validate:"required,email"`
		Password string `json:"password" form:"password" validate:"required,min=8"`
	}

	sessionAPI := app.Engine.Group("/")
	sessionAPI.Use(csrf.Middleware(csrf.Options{}))

	sessionAPI.POST("/login", middleware.LoginRateLimit(), func(c *gin.Context) {
		var body loginBody
		if err := validate.BindJSON(c, &body); err != nil {
			validate.Abort(c, err)
			return
		}
		u, ok := users[body.Email]
		if !ok || auth.CheckPassword(u.PasswordHash, body.Password) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
			return
		}
		au := auth.User{ID: u.ID, Email: u.Email, Roles: u.Roles}
		if err := authenticator.LoginSession(c, au); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "user": au})
	})

	sessionAPI.POST("/logout", authenticator.RequireAuthSession(), func(c *gin.Context) {
		_ = authenticator.LogoutSession(c)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	app.Engine.GET("/me", authenticator.RequireAuthSession(), func(c *gin.Context) {
		u, _ := auth.UserFrom(c)
		c.JSON(http.StatusOK, gin.H{"user": u})
	})

	app.Engine.GET("/admin",
		authenticator.RequireAuthSession(),
		rbac.RequirePermission(policies, "users:manage"),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"secret": "admin-only"})
		},
	)

	app.Engine.POST("/token", func(c *gin.Context) {
		var body loginBody
		if err := validate.BindJSON(c, &body); err != nil {
			validate.Abort(c, err)
			return
		}
		u, ok := users[body.Email]
		if !ok || auth.CheckPassword(u.PasswordHash, body.Password) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
			return
		}
		au := auth.User{ID: u.ID, Email: u.Email, Roles: u.Roles}
		tok, exp, err := authenticator.IssueAccessToken(au)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"access_token": tok,
			"token_type":   "Bearer",
			"expires_at":   exp.UTC().Format(time.RFC3339),
		})
	})

	app.Engine.GET("/jwt/me", authenticator.RequireJWT(), func(c *gin.Context) {
		u, _ := auth.UserFrom(c)
		c.JSON(http.StatusOK, gin.H{"user": u})
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("irmik auth demo on http://%s\n", cfg.Addr())
	if err := app.Run(ctx); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "auth example: %v\n", err)
	os.Exit(1)
}
