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
	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/sse"
	"github.com/boracomet/go-irmik/irmik/ws"
)

func main() {
	cfg, err := config.Load("irmik.yaml")
	if err != nil {
		fatal(err)
	}
	// Long-lived streams must not hit the default write timeout.
	cfg.Server.WriteTimeout = 0
	cfg.Server.ReadTimeout = 0

	app, err := irmik.New(cfg)
	if err != nil {
		fatal(err)
	}

	wsOpts := ws.Options{AllowedOrigins: cfg.Realtime.AllowedOrigins, Development: cfg.IsDev()}
	sseHub := sse.NewHub()
	chatHub := ws.NewHub(wsOpts)
	chatHub.Start()
	defer chatHub.Close()

	app.Engine.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		_, _ = c.Writer.Write([]byte(indexHTML))
	})

	// SSE: one-second clock until the client disconnects.
	app.Engine.GET("/sse/clock", sse.Handler(sse.Options{
		Heartbeat: 15 * time.Second,
	}, func(s *sse.Stream) error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		n := 0
		for {
			select {
			case <-s.Done():
				return nil
			case t := <-ticker.C:
				n++
				if err := s.Event("tick", gin.H{
					"n":    n,
					"time": t.UTC().Format(time.RFC3339),
				}); err != nil {
					return err
				}
			}
		}
	}))

	// SSE: fan-out from a shared hub (demo producer ticks every 2s).
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for now := range t.C {
			sseHub.Broadcast(sse.Event{
				Event: "pulse",
				Data:  fmt.Sprintf(`{"at":%q,"subscribers":%d}`, now.UTC().Format(time.RFC3339), sseHub.Len()),
			})
		}
	}()
	app.Engine.GET("/sse/stream", sse.Handler(sse.Options{Heartbeat: 20 * time.Second}, func(s *sse.Stream) error {
		sseHub.Attach(s)
		return nil
	}))

	// WebSocket echo.
	app.Engine.GET("/ws/echo", ws.Handler(wsOpts, func(c *ws.Conn) error {
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				return err
			}
			out := append([]byte("echo: "), msg...)
			if err := c.WriteMessage(mt, out); err != nil {
				return err
			}
		}
	}))

	// WebSocket chat room (?room=lobby).
	app.Engine.GET("/ws/chat", chatHub.ServeHTTP)

	app.Engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"sse_subscribers": sseHub.Len(),
			"ws_clients":      chatHub.Len(),
		})
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("irmik realtime demo on http://%s\n", cfg.Addr())
	fmt.Println("  GET  /sse/clock   — SSE clock")
	fmt.Println("  GET  /sse/stream  — SSE hub fan-out")
	fmt.Println("  GET  /ws/echo     — WebSocket echo")
	fmt.Println("  GET  /ws/chat?room=lobby — WebSocket chat room")
	if err := app.Run(ctx); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "realtime example: %v\n", err)
	os.Exit(1)
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Irmik realtime</title>
  <style>
    body { font-family: ui-sans-serif, system-ui, sans-serif; max-width: 42rem; margin: 2rem auto; padding: 0 1rem; }
    h1 { font-size: 1.35rem; }
    section { margin: 1.5rem 0; }
    pre { background: #f4f4f5; padding: .75rem; overflow: auto; min-height: 4rem; }
    input, button { font: inherit; margin: .25rem .25rem .25rem 0; }
  </style>
</head>
<body>
  <h1>Irmik realtime demo</h1>
  <p>SSE clock / stream and WebSocket echo / chat. See <code>docs/realtime.md</code>.</p>

  <section>
    <h2>SSE clock</h2>
    <pre id="clock">connecting…</pre>
  </section>
  <section>
    <h2>SSE stream (hub)</h2>
    <pre id="stream">connecting…</pre>
  </section>
  <section>
    <h2>WS echo</h2>
    <input id="echoIn" placeholder="message" />
    <button id="echoSend">Send</button>
    <pre id="echoOut"></pre>
  </section>
  <section>
    <h2>WS chat (room=lobby)</h2>
    <input id="chatIn" placeholder="chat message" />
    <button id="chatSend">Send</button>
    <pre id="chatOut"></pre>
  </section>
  <script>
    const clock = document.getElementById('clock');
    const es1 = new EventSource('/sse/clock');
    es1.addEventListener('tick', e => { clock.textContent = e.data; });
    es1.onerror = () => { clock.textContent += '\n[disconnected]'; };

    const stream = document.getElementById('stream');
    const es2 = new EventSource('/sse/stream');
    es2.addEventListener('pulse', e => { stream.textContent = e.data + '\n' + stream.textContent; });

    const echoOut = document.getElementById('echoOut');
    const echoWS = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws/echo');
    echoWS.onmessage = e => { echoOut.textContent = e.data + '\n' + echoOut.textContent; };
    document.getElementById('echoSend').onclick = () => {
      const v = document.getElementById('echoIn').value;
      if (v) echoWS.send(v);
    };

    const chatOut = document.getElementById('chatOut');
    const chatWS = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws/chat?room=lobby');
    chatWS.onmessage = e => { chatOut.textContent = e.data + '\n' + chatOut.textContent; };
    document.getElementById('chatSend').onclick = () => {
      const v = document.getElementById('chatIn').value;
      if (v) chatWS.send(v);
    };
  </script>
</body>
</html>
`
