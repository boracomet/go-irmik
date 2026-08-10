// Package openapi provides a lightweight OpenAPI 3 document builder and Gin
// serve helper. No swaggo/code generation dependency — annotate routes in Go.
//
// Serve JSON with Doc.Mount, and optionally a CDN-backed Swagger UI via
// SwaggerUIHandler / MountSwagger (no vendored swagger-ui assets in this module).
//
//	doc := openapi.New("API", "1.0.0")
//	doc.Mount(r, "/openapi.json")
//	openapi.MountSwagger(r, "/docs", "/openapi.json")
//
// Experimental: covers common CRUD docs; not a full OpenAPI validator.
package openapi

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Doc is an OpenAPI 3.0 document subset (JSON-serializable).
type Doc struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Servers    []Server            `json:"servers,omitempty"`
	Paths      map[string]PathItem `json:"paths"`
	Components *Components         `json:"components,omitempty"`

	mu sync.Mutex
}

// Info is OpenAPI info.
type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// Server is an OpenAPI server entry.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// PathItem holds operations for a path.
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

// Operation describes one HTTP operation.
type Operation struct {
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

// Parameter is a path/query/header parameter.
type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required,omitempty"`
	Schema   Schema `json:"schema"`
}

// RequestBody describes a request body.
type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}

// Response describes a response.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// MediaType wraps a schema.
type MediaType struct {
	Schema Schema `json:"schema"`
}

// Schema is a JSON Schema subset.
type Schema struct {
	Type       string            `json:"type,omitempty"`
	Format     string            `json:"format,omitempty"`
	Properties map[string]Schema `json:"properties,omitempty"`
	Items      *Schema           `json:"items,omitempty"`
	Ref        string            `json:"$ref,omitempty"`
}

// Components holds reusable schemas.
type Components struct {
	Schemas map[string]Schema `json:"schemas,omitempty"`
}

// New creates a document with openapi 3.0.3.
func New(title, version string) *Doc {
	return &Doc{
		OpenAPI: "3.0.3",
		Info:    Info{Title: title, Version: version},
		Paths:   map[string]PathItem{},
	}
}

// Add merges op into path for method (GET, POST, …).
func (d *Doc) Add(path, method string, op Operation) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if op.Responses == nil {
		op.Responses = map[string]Response{
			"200": {Description: "OK"},
		}
	}
	item := d.Paths[path]
	switch strings.ToUpper(method) {
	case http.MethodGet:
		item.Get = &op
	case http.MethodPost:
		item.Post = &op
	case http.MethodPut:
		item.Put = &op
	case http.MethodPatch:
		item.Patch = &op
	case http.MethodDelete:
		item.Delete = &op
	}
	d.Paths[path] = item
}

// Handler serves the OpenAPI document as JSON.
func (d *Doc) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, d)
	}
}

// Mount registers GET path (e.g. "/openapi.json") on r.
func (d *Doc) Mount(r gin.IRoutes, path string) {
	if path == "" {
		path = "/openapi.json"
	}
	r.GET(path, d.Handler())
}

// MountSwagger registers a CDN-backed Swagger UI page that loads this doc's JSON
// from specPath (default "/openapi.json"). Also mounts the JSON if not already
// registered at the same path — call Mount separately when you want a custom JSON path.
//
//	doc.Mount(r, "/openapi.json")
//	doc.MountSwagger(r, "/docs", "/openapi.json")
func (d *Doc) MountSwagger(r gin.IRoutes, uiPath, specPath string) {
	if specPath == "" {
		specPath = "/openapi.json"
	}
	MountSwagger(r, uiPath, specPath)
}

// SwaggerUIHandler returns HTML that loads Swagger UI from a public CDN and
// points it at specURL (absolute path or full URL, e.g. "/openapi.json").
//
// This keeps heavy swagger-ui assets out of the Go module. For fully offline
// installs, vendor swagger-ui dist yourself and serve static files that fetch
// the same OpenAPI JSON endpoint.
func SwaggerUIHandler(specURL string) gin.HandlerFunc {
	if specURL == "" {
		specURL = "/openapi.json"
	}
	html := swaggerUIHTML(specURL)
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}
}

// MountSwagger registers GET path (default "/docs") serving Swagger UI for specURL.
func MountSwagger(r gin.IRoutes, path, specURL string) {
	if path == "" {
		path = "/docs"
	}
	if specURL == "" {
		specURL = "/openapi.json"
	}
	r.GET(path, SwaggerUIHandler(specURL))
}

func swaggerUIHTML(specURL string) string {
	// Escape for use inside a JS string literal.
	escaped := strings.ReplaceAll(specURL, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "` + escaped + `",
        dom_id: "#swagger-ui",
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`
}

// JSONSchemaObject is a helper for object schemas.
func JSONSchemaObject(props map[string]Schema) Schema {
	return Schema{Type: "object", Properties: props}
}
