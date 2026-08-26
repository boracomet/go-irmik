package sse_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/sse"
)

func init() { gin.SetMode(gin.TestMode) }

func TestFormat(t *testing.T) {
	got := sse.Format(sse.Event{
		ID:    "42",
		Event: "tick",
		Data:  "hello\nworld",
		Retry: 3 * time.Second,
	})
	want := "id: 42\nevent: tick\nretry: 3000\ndata: hello\ndata: world\n\n"
	if got != want {
		t.Fatalf("Format()\n got %q\nwant %q", got, want)
	}
}

func TestFormatSanitizesID(t *testing.T) {
	got := sse.Format(sse.Event{ID: "a\nb", Data: "x"})
	if strings.Contains(got, "\nid: b") || strings.Contains(got, "id: a\nb") {
		t.Fatalf("id not sanitized: %q", got)
	}
	if !strings.Contains(got, "id: ab\n") {
		t.Fatalf("expected sanitized id, got %q", got)
	}
}

func TestStreamWritesAndCancelsOnClose(t *testing.T) {
	r := gin.New()
	r.GET("/sse", func(c *gin.Context) {
		s, err := sse.New(c, sse.Options{})
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer s.Close()
		if err := s.Event("hello", map[string]string{"msg": "hi"}); err != nil {
			t.Errorf("Event: %v", err)
			return
		}
		if err := s.Comment("ping"); err != nil {
			t.Errorf("Comment: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "event: hello\n") {
		t.Fatalf("missing event: %q", body)
	}
	if !strings.Contains(body, `data: {"msg":"hi"}`) {
		t.Fatalf("missing data: %q", body)
	}
	if !strings.Contains(body, ": ping\n") {
		t.Fatalf("missing comment: %q", body)
	}
}

func TestHandlerHeartbeatAndDisconnect(t *testing.T) {
	r := gin.New()
	r.GET("/sse", sse.Handler(sse.Options{Heartbeat: 20 * time.Millisecond}, func(s *sse.Stream) error {
		ticker := time.NewTicker(15 * time.Millisecond)
		defer ticker.Stop()
		n := 0
		for {
			select {
			case <-s.Done():
				return nil
			case <-ticker.C:
				n++
				if err := s.Data("tick"); err != nil {
					return err
				}
				if n >= 2 {
					s.Close()
				}
			}
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "data: tick") {
		t.Fatalf("expected ticks, got %q", body)
	}
}

func TestHubBroadcast(t *testing.T) {
	h := sse.NewHub()
	ch := h.Subscribe(4)
	defer h.Unsubscribe(ch)

	h.Broadcast(sse.Event{Event: "x", Data: "1"})
	select {
	case e := <-ch:
		if e.Event != "x" || e.Data != "1" {
			t.Fatalf("got %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
	if h.Len() != 1 {
		t.Fatalf("Len = %d", h.Len())
	}
}

func TestSSEClearsWriteDeadline(t *testing.T) {
	r := gin.New()
	writeErr := make(chan error, 1)
	r.GET("/sse", func(c *gin.Context) {
		s, err := sse.New(c, sse.Options{})
		if err != nil {
			writeErr <- err
			return
		}
		defer s.Close()
		time.Sleep(120 * time.Millisecond)
		writeErr <- s.Data("still-alive")
	})

	srv := httptest.NewUnstartedServer(r)
	srv.Config.WriteTimeout = 40 * time.Millisecond
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	select {
	case err := <-writeErr:
		if err != nil {
			t.Fatalf("SSE write after WriteTimeout: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not finish")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "data: still-alive") {
		t.Fatalf("missing event, got %q", body)
	}
}

func TestStreamOverHTTP(t *testing.T) {
	r := gin.New()
	hub := sse.NewHub()
	r.GET("/sse", sse.Handler(sse.Options{}, func(s *sse.Stream) error {
		go func() {
			time.Sleep(10 * time.Millisecond)
			hub.Broadcast(sse.Event{Data: "fanout"})
		}()
		hub.Attach(s)
		return nil
	}))

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	sc := bufio.NewScanner(resp.Body)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout reading SSE")
		default:
		}
		if !sc.Scan() {
			t.Fatalf("scan ended: %v", sc.Err())
		}
		line := sc.Text()
		if strings.HasPrefix(line, "data: fanout") {
			return
		}
	}
}
