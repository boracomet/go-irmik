// Package testkit provides HTTP test helpers for Gin and Irmik-style apps.
package testkit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Server wraps a gin.Engine for table-driven HTTP tests.
type Server struct {
	Engine *gin.Engine
	T      testing.TB
}

// New creates a Server with a bare gin.Engine (no default middleware).
func New(t testing.TB) *Server {
	t.Helper()
	return &Server{Engine: gin.New(), T: t}
}

// NewWithEngine wraps an existing engine.
func NewWithEngine(t testing.TB, e *gin.Engine) *Server {
	t.Helper()
	return &Server{Engine: e, T: t}
}

// Request is a fluent HTTP request builder.
type Request struct {
	s       *Server
	method  string
	path    string
	headers http.Header
	body    io.Reader
	form    url.Values
}

// GET starts a GET request.
func (s *Server) GET(path string) *Request {
	return &Request{s: s, method: http.MethodGet, path: path, headers: http.Header{}}
}

// POST starts a POST request.
func (s *Server) POST(path string) *Request {
	return &Request{s: s, method: http.MethodPost, path: path, headers: http.Header{}}
}

// PUT starts a PUT request.
func (s *Server) PUT(path string) *Request {
	return &Request{s: s, method: http.MethodPut, path: path, headers: http.Header{}}
}

// DELETE starts a DELETE request.
func (s *Server) DELETE(path string) *Request {
	return &Request{s: s, method: http.MethodDelete, path: path, headers: http.Header{}}
}

// Header sets a request header.
func (r *Request) Header(k, v string) *Request {
	r.headers.Set(k, v)
	return r
}

// JSON sets a JSON body and Content-Type.
func (r *Request) JSON(v any) *Request {
	b, err := json.Marshal(v)
	if err != nil {
		r.s.T.Fatalf("testkit: json marshal: %v", err)
	}
	r.body = bytes.NewReader(b)
	r.headers.Set("Content-Type", "application/json")
	return r
}

// Form sets application/x-www-form-urlencoded body.
func (r *Request) Form(values map[string]string) *Request {
	r.form = url.Values{}
	for k, v := range values {
		r.form.Set(k, v)
	}
	r.body = strings.NewReader(r.form.Encode())
	r.headers.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// Body sets a raw body.
func (r *Request) Body(body io.Reader) *Request {
	r.body = body
	return r
}

// Response is the recorded result.
type Response struct {
	Code   int
	Header http.Header
	Body   []byte
	T      testing.TB
}

// Do executes the request.
func (r *Request) Do() *Response {
	r.s.T.Helper()
	req := httptest.NewRequest(r.method, r.path, r.body)
	for k, vs := range r.headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	r.s.Engine.ServeHTTP(rec, req)
	return &Response{Code: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes(), T: r.s.T}
}

// ExpectStatus fatals unless code matches.
func (r *Response) ExpectStatus(code int) *Response {
	r.T.Helper()
	if r.Code != code {
		r.T.Fatalf("status=%d want=%d body=%s", r.Code, code, r.Body)
	}
	return r
}

// JSON unmarshals the body into dst.
func (r *Response) JSON(dst any) *Response {
	r.T.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		r.T.Fatalf("json unmarshal: %v body=%s", err, r.Body)
	}
	return r
}

// String returns the body as string.
func (r *Response) String() string { return string(r.Body) }
