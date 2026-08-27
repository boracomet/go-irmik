package ws_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/boracomet/go-irmik/irmik/ws"
)

func init() { gin.SetMode(gin.TestMode) }

func TestUpgradeAndEcho(t *testing.T) {
	r := gin.New()
	r.GET("/ws", ws.Handler(ws.Options{Development: true}, func(c *ws.Conn) error {
		_, msg, err := c.ReadMessage()
		if err != nil {
			return err
		}
		return c.WriteMessage(websocket.TextMessage, append([]byte("echo:"), msg...))
	}))

	srv := httptest.NewServer(r)
	defer srv.Close()

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "echo:hi" {
		t.Fatalf("got %q", got)
	}
}

func TestHubRoomBroadcast(t *testing.T) {
	hub := ws.NewHub(ws.Options{Development: true})
	hub.Start()
	defer hub.Close()

	r := gin.New()
	r.GET("/ws", hub.ServeHTTP)

	srv := httptest.NewServer(r)
	defer srv.Close()

	base := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	a, _, err := websocket.DefaultDialer.Dial(base+"?room=chat", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	b, _, err := websocket.DefaultDialer.Dial(base+"?room=chat", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()
	outsider, _, err := websocket.DefaultDialer.Dial(base+"?room=other", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = outsider.Close() }()

	// Wait for registrations
	deadline := time.Now().Add(2 * time.Second)
	for hub.Len() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.Len() < 3 {
		t.Fatalf("expected 3 clients, got %d", hub.Len())
	}

	if err := a.WriteMessage(websocket.TextMessage, []byte("hello-room")); err != nil {
		t.Fatal(err)
	}

	_ = b.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := b.ReadMessage()
	if err != nil {
		t.Fatalf("b read: %v", err)
	}
	if string(got) != "hello-room" {
		t.Fatalf("b got %q", got)
	}

	_ = outsider.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if _, _, err := outsider.ReadMessage(); err == nil {
		t.Fatal("outsider should not receive room message")
	}
}

func TestCheckOriginRejects(t *testing.T) {
	r := gin.New()
	r.GET("/ws", ws.Handler(ws.Options{
		AllowedOrigins: []string{"http://allowed.example"},
	}, func(c *ws.Conn) error {
		return nil
	}))

	srv := httptest.NewServer(r)
	defer srv.Close()

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(u, http.Header{
		"Origin": []string{"http://evil.example"},
	})
	if err == nil {
		t.Fatal("expected origin rejection")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		// gorilla returns 403 on CheckOrigin failure
		t.Logf("status=%d err=%v", resp.StatusCode, err)
	}
}

func TestCheckOriginRejectsEmptyOriginWithAllowlist(t *testing.T) {
	r := gin.New()
	r.GET("/ws", ws.Handler(ws.Options{
		AllowedOrigins: []string{"http://allowed.example"},
	}, func(c *ws.Conn) error { return nil }))

	srv := httptest.NewServer(r)
	defer srv.Close()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	if _, _, err := websocket.DefaultDialer.Dial(u, nil); err == nil {
		t.Fatal("expected missing Origin rejection")
	}
}

func TestCheckOriginRejectsEmptyAllowlistOutsideDevelopment(t *testing.T) {
	r := gin.New()
	r.GET("/ws", ws.Handler(ws.Options{}, func(c *ws.Conn) error { return nil }))

	srv := httptest.NewServer(r)
	defer srv.Close()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	if _, _, err := websocket.DefaultDialer.Dial(u, http.Header{"Origin": []string{"http://app.example"}}); err == nil {
		t.Fatal("expected empty allowlist rejection")
	}
}

func TestHubServeHTTPWithoutStart(t *testing.T) {
	hub := ws.NewHub(ws.Options{Development: true})
	defer hub.Close()

	r := gin.New()
	r.GET("/ws", hub.ServeHTTP)
	srv := httptest.NewServer(r)
	defer srv.Close()

	done := make(chan error, 1)
	go func() {
		u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(u, nil)
		if err != nil {
			done <- err
			return
		}
		defer func() { _ = conn.Close() }()

		deadline := time.Now().Add(2 * time.Second)
		for hub.Len() < 1 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if hub.Len() < 1 {
			done <- fmt.Errorf("client not registered")
			return
		}
		hub.Broadcast([]byte("hello-lazy"))
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, got, err := conn.ReadMessage()
		if err != nil {
			done <- err
			return
		}
		if string(got) != "hello-lazy" {
			done <- fmt.Errorf("got %q", got)
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ServeHTTP deadlocked without Start()")
	}
}

func TestHubRegisterAfterCloseDoesNotDeadlock(t *testing.T) {
	hub := ws.NewHub(ws.Options{Development: true})
	hub.Close()

	r := gin.New()
	r.GET("/ws", hub.ServeHTTP)
	srv := httptest.NewServer(r)
	defer srv.Close()

	done := make(chan error, 1)
	go func() {
		u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(u, nil)
		if conn != nil {
			_ = conn.Close()
		}
		// Dial may succeed then get closed, or fail; either is fine as long as we return.
		_ = err
		done <- nil
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ServeHTTP after Close deadlocked")
	}
}
