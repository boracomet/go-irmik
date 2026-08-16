// Package upload provides multipart file upload helpers with size and MIME limits.
//
// Opt-in: import only when you need uploads. Nothing here is wired by irmik.New.
package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Options configures multipart parsing and acceptance rules.
type Options struct {
	// MaxBytes is the maximum request body size (default 32 MiB).
	MaxBytes int64
	// Field is the form field name for the file (default "file").
	Field string
	// AllowedMIME, when non-empty, restricts Content-Type / detected MIME.
	AllowedMIME []string
	// AllowAnyMIME disables the conservative default allowlist.
	AllowAnyMIME bool
	// DestDir is where Save writes files. Required for Save / Handler.
	DestDir string
}

func (o Options) withDefaults() Options {
	if o.MaxBytes <= 0 {
		o.MaxBytes = 32 << 20
	}
	if o.Field == "" {
		o.Field = "file"
	}
	if len(o.AllowedMIME) == 0 && !o.AllowAnyMIME {
		o.AllowedMIME = []string{"image/jpeg", "image/png", "image/webp", "application/pdf"}
	}
	return o
}

// File is a successfully accepted upload.
type File struct {
	Filename    string
	Size        int64
	ContentType string
	// Path is set after Save.
	Path   string
	Header *multipart.FileHeader
}

// ErrTooLarge is returned when the upload exceeds MaxBytes.
var ErrTooLarge = fmt.Errorf("upload: file too large")

// ErrMIMERejected is returned when Content-Type is not allowed.
var ErrMIMERejected = fmt.Errorf("upload: mime type not allowed")

// Parse reads a single file from the multipart form without writing to disk.
func Parse(c *gin.Context, opts Options) (*File, error) {
	opts = opts.withDefaults()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, opts.MaxBytes)
	if err := c.Request.ParseMultipartForm(opts.MaxBytes); err != nil {
		if strings.Contains(err.Error(), "request body too large") ||
			strings.Contains(err.Error(), "http: request body too large") {
			return nil, ErrTooLarge
		}
		return nil, fmt.Errorf("upload: parse multipart: %w", err)
	}
	fh, err := c.FormFile(opts.Field)
	if err != nil {
		return nil, fmt.Errorf("upload: form file %q: %w", opts.Field, err)
	}
	if fh.Size > opts.MaxBytes {
		return nil, ErrTooLarge
	}
	ct := fh.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		if detected, derr := sniffMIME(fh); derr == nil && detected != "" {
			ct = detected
		}
	}
	if !mimeAllowed(ct, opts.AllowedMIME) {
		return nil, fmt.Errorf("%w: %s", ErrMIMERejected, ct)
	}
	return &File{
		Filename:    filepath.Base(fh.Filename),
		Size:        fh.Size,
		ContentType: ct,
		Header:      fh,
	}, nil
}

// Save parses the upload and writes it under DestDir using a safe base name.
// The returned File.Path is absolute when possible.
func Save(c *gin.Context, opts Options) (*File, error) {
	opts = opts.withDefaults()
	if opts.DestDir == "" {
		return nil, fmt.Errorf("upload: DestDir is required")
	}
	f, err := Parse(c, opts)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.DestDir, 0o755); err != nil {
		return nil, fmt.Errorf("upload: mkdir: %w", err)
	}
	name := safeName(f.Filename)
	dest := filepath.Join(opts.DestDir, name)
	src, err := f.Header.Open()
	if err != nil {
		return nil, fmt.Errorf("upload: open: %w", err)
	}
	defer func() { _ = src.Close() }()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("upload: create: %w", err)
	}
	defer func() { _ = out.Close() }()

	n, err := io.Copy(out, io.LimitReader(src, opts.MaxBytes+1))
	if err != nil {
		_ = os.Remove(dest)
		return nil, fmt.Errorf("upload: write: %w", err)
	}
	if n > opts.MaxBytes {
		_ = os.Remove(dest)
		return nil, ErrTooLarge
	}
	f.Size = n
	f.Path = dest
	f.Filename = name
	return f, nil
}

// Handler returns a Gin handler that saves the upload and responds with JSON metadata.
func Handler(opts Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		f, err := Save(c, opts)
		if err != nil {
			status := http.StatusBadRequest
			if err == ErrTooLarge {
				status = http.StatusRequestEntityTooLarge
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"filename":     f.Filename,
			"size":         f.Size,
			"content_type": f.ContentType,
		})
	}
}

func safeName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "upload.bin"
	}
	return name
}

func mimeAllowed(ct string, allowed []string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == ct {
			return true
		}
		if strings.HasSuffix(a, "/*") {
			prefix := strings.TrimSuffix(a, "/*")
			if strings.HasPrefix(ct, prefix+"/") {
				return true
			}
		}
	}
	return false
}

func sniffMIME(fh *multipart.FileHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}
