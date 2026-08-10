package testkit

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestServerJSON(t *testing.T) {
	s := New(t)
	s.Engine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	var out map[string]any
	s.GET("/ping").Do().ExpectStatus(200).JSON(&out)
	if out["ok"] != true {
		t.Fatalf("%v", out)
	}
}

func TestForm(t *testing.T) {
	s := New(t)
	s.Engine.POST("/f", func(c *gin.Context) {
		c.String(200, c.PostForm("name"))
	})
	res := s.POST("/f").Form(map[string]string{"name": "irmik"}).Do().ExpectStatus(200)
	if res.String() != "irmik" {
		t.Fatalf("%q", res.String())
	}
}
