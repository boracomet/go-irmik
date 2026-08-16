package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik"
	"github.com/boracomet/go-irmik/irmik/admin"
	"github.com/boracomet/go-irmik/irmik/api"
	"github.com/boracomet/go-irmik/irmik/auth"
	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/cors"
	"github.com/boracomet/go-irmik/irmik/csrf"
	"github.com/boracomet/go-irmik/irmik/db"
	_ "github.com/boracomet/go-irmik/irmik/db/sqlite"
	"github.com/boracomet/go-irmik/irmik/forms"
	"github.com/boracomet/go-irmik/irmik/htmx"
	"github.com/boracomet/go-irmik/irmik/middleware"
	"github.com/boracomet/go-irmik/irmik/paginate"
	"github.com/boracomet/go-irmik/irmik/rbac"
	rbacstore "github.com/boracomet/go-irmik/irmik/rbac/store"
	"github.com/boracomet/go-irmik/irmik/validate"
)

//go:embed templates/*.html
var templateFS embed.FS

type demoUser struct {
	ID           string
	Email        string
	PasswordHash string
	Roles        []string
}

type server struct {
	app   *irmik.App
	auth  *auth.Authenticator
	tmpl  *template.Template
	store *itemStore
	users map[string]demoUser
	byID  map[string]demoUser
	rbac  *rbac.Registry
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

	// CORS for Next.js / SPA clients calling the JWT API.
	app.Engine.Use(cors.Middleware(cors.Options{
		AllowOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowCredentials: false,
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
	}))

	dsn := cfg.Database.DSNOrURL()
	if dsn == "" {
		dsn = "file:irmik-admin-demo?mode=memory&cache=shared"
	}
	database, err := db.Open(db.Options{Driver: "sqlite", DSN: dsn})
	if err != nil {
		fatal(err)
	}
	defer database.Close()

	store, err := newItemStore(database.DB())
	if err != nil {
		fatal(err)
	}

	hash, err := auth.HashPasswordDefault("password123")
	if err != nil {
		fatal(err)
	}
	users := map[string]demoUser{
		"admin@example.com": {
			ID: "1", Email: "admin@example.com", PasswordHash: hash, Roles: []string{rbac.RoleAdmin},
		},
		"editor@example.com": {
			ID: "2", Email: "editor@example.com", PasswordHash: hash, Roles: []string{rbac.RoleEditor},
		},
	}
	byID := map[string]demoUser{}
	for _, u := range users {
		byID[u.ID] = u
	}

	// Persist RBAC in the same SQLite DB, then sync into an in-memory Registry.
	policyStore := rbacstore.NewSQL(database.DB(), "sqlite")
	if err := rbacstore.SeedPresets(context.Background(), policyStore, "items"); err != nil {
		fatal(err)
	}
	if err := policyStore.AssignRole(context.Background(), "1", rbac.RoleAdmin); err != nil {
		fatal(err)
	}
	if err := policyStore.AssignRole(context.Background(), "2", rbac.RoleEditor); err != nil {
		fatal(err)
	}
	policies, err := policyStore.LoadRegistry(context.Background())
	if err != nil {
		fatal(err)
	}

	adminSnippets, err := admin.ParseTemplates(nil)
	if err != nil {
		fatal(err)
	}
	tmpl, err := template.New("admin-demo").Funcs(rbac.FuncMap(policies)).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		fatal(err)
	}
	// Merge embedded catalog snippets (flash.html, list.html, …) into the app set.
	for _, name := range []string{"flash.html", "list.html", "form.html", "confirm_delete.html"} {
		if t := adminSnippets.Lookup(name); t != nil {
			if _, err := tmpl.AddParseTree(name, t.Tree); err != nil {
				fatal(err)
			}
		}
	}

	s := &server{
		app: app, auth: authenticator, tmpl: tmpl, store: store,
		users: users, byID: byID, rbac: policies,
	}

	app.Engine.Use(authenticator.InjectSessionUser(func(userID string) (*auth.User, error) {
		u, ok := byID[userID]
		if !ok {
			return nil, nil
		}
		return &auth.User{ID: u.ID, Email: u.Email, Roles: u.Roles}, nil
	}))

	// Browser routes: CSRF on unsafe methods.
	browser := app.Engine.Group("/")
	browser.Use(csrf.Middleware(csrf.Options{}))

	browser.GET("/", s.handleHome)
	browser.GET("/login", s.handleLoginGet)
	browser.POST("/login", middleware.LoginRateLimit(), s.handleLoginPost)
	browser.POST("/logout", authenticator.RequireAuthSession(), s.handleLogout)

	adminUI := browser.Group("/admin")
	adminUI.Use(func(c *gin.Context) {
		if authenticator.SessionUserID(c) == "" {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		if _, ok := auth.UserFrom(c); !ok {
			auth.SetUser(c, auth.User{ID: authenticator.SessionUserID(c)})
		}
		c.Next()
	}, rbac.RequirePermission(policies, rbac.Perm("items", "read")))
	adminUI.GET("", func(c *gin.Context) { c.Redirect(http.StatusSeeOther, "/admin/items") })
	adminUI.GET("/items", s.handleItemsList)
	adminUI.GET("/items/new", rbac.RequirePermission(policies, rbac.Perm("items", "write")), s.handleItemNew)
	adminUI.POST("/items", rbac.RequirePermission(policies, rbac.Perm("items", "write")), s.handleItemCreate)
	adminUI.GET("/items/:id/edit", rbac.RequirePermission(policies, rbac.Perm("items", "write")), s.handleItemEdit)
	adminUI.POST("/items/:id", rbac.RequirePermission(policies, rbac.Perm("items", "write")), s.handleItemUpdate)
	adminUI.GET("/items/:id/delete", rbac.RequirePermission(policies, rbac.Perm("items", "delete")), s.handleItemDeleteConfirm)
	adminUI.POST("/items/:id/delete", rbac.RequirePermission(policies, rbac.Perm("items", "delete")), s.handleItemDelete)

	// JWT API for external clients (Next.js, mobile, …).
	api.MountV1(app.Engine, func(v1 *gin.RouterGroup) {
		v1.POST("/token", middleware.LoginRateLimit(), s.handleAPIToken)
		v1.POST("/token/refresh", middleware.LoginRateLimit(), s.handleAPIRefresh)
		v1.POST("/token/revoke", authenticator.RequireJWT(), s.handleAPIRevoke)

		items := v1.Group("/items")
		items.Use(authenticator.RequireJWT())
		items.GET("", rbac.RequirePermission(policies, rbac.Perm("items", "read")), s.apiListItems)
		items.POST("", rbac.RequirePermission(policies, rbac.Perm("items", "write")), s.apiCreateItem)
		items.GET("/:id", rbac.RequirePermission(policies, rbac.Perm("items", "read")), s.apiGetItem)
		items.PUT("/:id", rbac.RequirePermission(policies, rbac.Perm("items", "write")), s.apiUpdateItem)
		items.PATCH("/:id", rbac.RequirePermission(policies, rbac.Perm("items", "write")), s.apiUpdateItem)
		items.DELETE("/:id", rbac.RequirePermission(policies, rbac.Perm("items", "delete")), s.apiDeleteItem)
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("irmik admin demo on http://%s\n", cfg.Addr())
	fmt.Println("  UI:  /login  →  /admin/items")
	fmt.Println("  API: /api/v1/token  /api/v1/items")
	fmt.Println("  RBAC: admin can delete; editor can create/edit (no delete)")
	if err := app.Run(ctx); err != nil {
		fatal(err)
	}
}

func (s *server) baseData(c *gin.Context) map[string]any {
	u, _ := auth.UserFrom(c)
	flash := admin.FlashData(c)
	return map[string]any{
		"User":  u,
		"CSRF":  forms.CSRFInput(csrf.Ensure(c)),
		"Flash": flash,
		"Title": "Irmik Admin",
	}
}

func (s *server) render(c *gin.Context, partial string, data map[string]any) {
	if data == nil {
		data = s.baseData(c)
	} else {
		base := s.baseData(c)
		for k, v := range base {
			if _, ok := data[k]; !ok {
				data[k] = v
			}
		}
	}
	data["Partial"] = partial
	if htmx.IsRequest(c) {
		_ = htmx.RenderPartial(c, s.tmpl, partial, data)
		return
	}
	_ = htmx.RenderPartial(c, s.tmpl, "shell", data)
}

func (s *server) handleHome(c *gin.Context) {
	if _, ok := auth.UserFrom(c); ok {
		c.Redirect(http.StatusSeeOther, "/admin/items")
		return
	}
	s.render(c, "page_home", nil)
}

func (s *server) handleLoginGet(c *gin.Context) {
	if _, ok := auth.UserFrom(c); ok {
		c.Redirect(http.StatusSeeOther, "/admin/items")
		return
	}
	s.render(c, "page_login", map[string]any{"Title": "Sign in"})
}

func (s *server) handleLoginPost(c *gin.Context) {
	type loginBody struct {
		Email    string `form:"email" validate:"required,email"`
		Password string `form:"password" validate:"required,min=8"`
	}
	var body loginBody
	if err := validate.BindForm(c, &body); err != nil {
		s.render(c, "page_login", map[string]any{"Title": "Sign in", "Error": "Invalid form", "Email": body.Email})
		return
	}
	u, ok := s.users[body.Email]
	if !ok || auth.CheckPassword(u.PasswordHash, body.Password) != nil {
		s.render(c, "page_login", map[string]any{"Title": "Sign in", "Error": "Invalid credentials", "Email": body.Email})
		return
	}
	au := auth.User{ID: u.ID, Email: u.Email, Roles: u.Roles}
	if err := s.auth.LoginSession(c, au); err != nil {
		s.render(c, "page_login", map[string]any{"Title": "Sign in", "Error": err.Error()})
		return
	}
	admin.SetFlash(c, admin.FlashSuccess, "Signed in")
	c.Redirect(http.StatusSeeOther, "/admin/items")
}

func (s *server) handleLogout(c *gin.Context) {
	_ = s.auth.LogoutSession(c)
	c.Redirect(http.StatusSeeOther, "/login")
}

func (s *server) handleItemsList(c *gin.Context) {
	p := paginate.Parse(c, paginate.Options{
		DefaultPerPage: 20,
		MaxPerPage:     100,
		DefaultSort:    "id",
		DefaultOrder:   "desc",
		SortWhitelist:  []string{"id", "title", "created_at", "updated_at"},
	})
	items, total, err := s.store.list(c.Request.Context(), p)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	s.render(c, "items_list_partial", map[string]any{
		"Title":   "Items",
		"Items":   items,
		"Q":       p.Q,
		"PageNum": p.Page,
		"Total":   total,
	})
}

type itemForm struct {
	Title string `form:"title" json:"title" validate:"required,min=1,max=200"`
	Body  string `form:"body" json:"body" validate:"max=10000"`
}

func (s *server) handleItemNew(c *gin.Context) {
	s.render(c, "items_form_partial", map[string]any{
		"Title":     "New item",
		"FormTitle": "New item",
		"Action":    "/admin/items",
		"Item":      Item{},
	})
}

func (s *server) handleItemCreate(c *gin.Context) {
	var form itemForm
	if err := validate.BindForm(c, &form); err != nil {
		ve, _ := validate.AsErrors(err)
		s.render(c, "items_form_partial", map[string]any{
			"Title": "New item", "FormTitle": "New item", "Action": "/admin/items",
			"Item": Item{Title: form.Title, Body: form.Body}, "Errors": ve,
		})
		return
	}
	if _, err := s.store.create(c.Request.Context(), form.Title, form.Body); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	admin.FlashHX(c, admin.FlashSuccess, "Item created")
	admin.RenderOrRedirect(c, "/admin/items", func(c *gin.Context) error {
		s.handleItemsList(c)
		return nil
	})
}

func (s *server) handleItemEdit(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	it, err := s.store.get(c.Request.Context(), id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	s.render(c, "items_form_partial", map[string]any{
		"Title": "Edit item", "FormTitle": "Edit item",
		"Action": fmt.Sprintf("/admin/items/%d", id), "Item": it,
	})
}

func (s *server) handleItemUpdate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	var form itemForm
	if err := validate.BindForm(c, &form); err != nil {
		ve, _ := validate.AsErrors(err)
		s.render(c, "items_form_partial", map[string]any{
			"Title": "Edit item", "FormTitle": "Edit item",
			"Action": fmt.Sprintf("/admin/items/%d", id),
			"Item":   Item{ID: id, Title: form.Title, Body: form.Body}, "Errors": ve,
		})
		return
	}
	if _, err := s.store.update(c.Request.Context(), id, form.Title, form.Body); err != nil {
		if err == sql.ErrNoRows {
			c.Status(http.StatusNotFound)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	admin.FlashHX(c, admin.FlashSuccess, "Item updated")
	admin.RenderOrRedirect(c, "/admin/items", func(c *gin.Context) error {
		s.handleItemsList(c)
		return nil
	})
}

func (s *server) handleItemDeleteConfirm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	it, err := s.store.get(c.Request.Context(), id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	s.render(c, "items_delete_partial", map[string]any{"Title": "Delete item", "Item": it})
}

func (s *server) handleItemDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if err := s.store.delete(c.Request.Context(), id); err != nil {
		if err == sql.ErrNoRows {
			c.Status(http.StatusNotFound)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	admin.FlashHX(c, admin.FlashSuccess, "Item deleted")
	admin.RenderOrRedirect(c, "/admin/items", func(c *gin.Context) error {
		s.handleItemsList(c)
		return nil
	})
}

func (s *server) handleAPIToken(c *gin.Context) {
	type loginBody struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
	}
	var body loginBody
	if err := api.BindJSON(c, &body); err != nil {
		api.AbortValidation(c, err)
		return
	}
	u, ok := s.users[body.Email]
	if !ok || auth.CheckPassword(u.PasswordHash, body.Password) != nil {
		api.Abort(c, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	au := auth.User{ID: u.ID, Email: u.Email, Roles: u.Roles}
	pair, err := s.auth.IssueTokenPair(au)
	if err != nil {
		api.Internal(c, err.Error())
		return
	}
	api.JSON(c, http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token_type":    "Bearer",
		"expires_at":    pair.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *server) handleAPIRefresh(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	if err := api.BindJSON(c, &body); err != nil {
		api.AbortValidation(c, err)
		return
	}
	for _, u := range s.byID {
		pair, refreshErr := s.auth.Refresh(body.RefreshToken, auth.User{ID: u.ID, Email: u.Email, Roles: u.Roles})
		if refreshErr == nil {
			api.JSON(c, http.StatusOK, pair)
			return
		}
	}
	api.Abort(c, http.StatusUnauthorized, "unauthorized", "unauthorized")
}

func (s *server) handleAPIRevoke(c *gin.Context) {
	u, _ := auth.UserFrom(c)
	s.auth.RevokeUser(u.ID)
	api.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func (s *server) apiListItems(c *gin.Context) {
	p := paginate.Parse(c, paginate.Options{
		DefaultPerPage: 20,
		MaxPerPage:     100,
		DefaultSort:    "id",
		DefaultOrder:   "desc",
		SortWhitelist:  []string{"id", "title", "created_at", "updated_at"},
	})
	items, total, err := s.store.list(c.Request.Context(), p)
	if err != nil {
		api.Internal(c, err.Error())
		return
	}
	api.JSON(c, http.StatusOK, gin.H{
		"data": items,
		"meta": gin.H{"page": p.Page, "per_page": p.PerPage, "total": total, "q": p.Q},
	})
}

func (s *server) apiCreateItem(c *gin.Context) {
	var form itemForm
	if err := api.BindJSON(c, &form); err != nil {
		api.AbortValidation(c, err)
		return
	}
	it, err := s.store.create(c.Request.Context(), form.Title, form.Body)
	if err != nil {
		api.Internal(c, err.Error())
		return
	}
	api.JSON(c, http.StatusCreated, it)
}

func (s *server) apiGetItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		api.NotFound(c, "item not found")
		return
	}
	it, err := s.store.get(c.Request.Context(), id)
	if err != nil {
		api.NotFound(c, "item not found")
		return
	}
	api.JSON(c, http.StatusOK, it)
}

func (s *server) apiUpdateItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		api.NotFound(c, "item not found")
		return
	}
	var form itemForm
	if err := api.BindJSON(c, &form); err != nil {
		api.AbortValidation(c, err)
		return
	}
	it, err := s.store.update(c.Request.Context(), id, form.Title, form.Body)
	if err != nil {
		if err == sql.ErrNoRows {
			api.NotFound(c, "item not found")
			return
		}
		api.Internal(c, err.Error())
		return
	}
	api.JSON(c, http.StatusOK, it)
}

func (s *server) apiDeleteItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		api.NotFound(c, "item not found")
		return
	}
	if err := s.store.delete(c.Request.Context(), id); err != nil {
		if err == sql.ErrNoRows {
			api.NotFound(c, "item not found")
			return
		}
		api.Internal(c, err.Error())
		return
	}
	api.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "admin example: %v\n", err)
	os.Exit(1)
}
