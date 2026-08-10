package upload

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSaveAndMIME(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, "hello irmik"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	r := httptest.NewRequest(http.MethodPost, "/up", body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = r

	f, err := Save(c, Options{
		DestDir:     dir,
		MaxBytes:    1024,
		AllowedMIME: []string{"text/plain", "application/octet-stream"},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if f.Filename != "hello.txt" {
		t.Fatalf("filename = %q", f.Filename)
	}
	data, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello irmik" {
		t.Fatalf("content = %q", data)
	}
}

func TestMIMERejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "x.bin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte{0x00, 0x01, 0x02})
	_ = w.Close()

	r := httptest.NewRequest(http.MethodPost, "/up", body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = r

	_, err = Parse(c, Options{AllowedMIME: []string{"image/png"}, MaxBytes: 1024})
	if err == nil {
		t.Fatal("expected MIME rejection")
	}
}

func TestSafeName(t *testing.T) {
	if got := safeName("../../etc/passwd"); got == "../../etc/passwd" {
		t.Fatalf("unsafe name leaked: %q", got)
	}
	if got := safeName(""); got != "upload.bin" {
		t.Fatalf("empty = %q", got)
	}
}
