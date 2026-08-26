# Realtime: SSE & WebSocket

Optional packages for long-lived connections on Gin. They do not auto-mount on `irmik.App` — register handlers on `app.Engine` like any other route.

| Package | Role |
|---------|------|
| [`irmik/sse`](../irmik/sse) | Server-Sent Events: framing, flush, heartbeat, optional broadcast Hub |
| [`irmik/ws`](../irmik/ws) | WebSocket upgrade (`gorilla/websocket`), Hub with rooms, ping/pong pumps |

## Server timeouts

`http.Server.WriteTimeout` applies from the moment headers are written. Irmik’s default is `30s`.

- **SSE:** `sse.New` clears the per-connection write deadline (`ResponseController.SetWriteDeadline(zero)`), so the default WriteTimeout should not kill an open stream. Heartbeats still help proxies.
- **WebSocket:** the upgrade hijacks the connection; WriteTimeout does not apply after hijack.
- **IdleTimeout:** `irmik.App` sets `http.Server.IdleTimeout` (default `60s`) for keep-alive connections between requests. It does not bound an active SSE write.

You can still set `writeTimeout: 0s` if a reverse proxy or custom streaming handler needs it:

```yaml
server:
  writeTimeout: 0s
  readTimeout: 0s
  idleTimeout: 60s
```

## Config / env

```yaml
realtime:
  allowedOrigins:
    - http://localhost:8080
    - https://app.example.com
```

| Variable | Field |
|----------|--------|
| `IRMIK_WS_ALLOWED_ORIGINS` | comma-separated `realtime.allowedOrigins` |

Empty origins are allowed only when `ws.Options.Development` is true. Production
must configure explicit origins, and a request without an `Origin` header is
rejected when an allowlist is configured; non-browser clients need an explicit
`CheckOrigin` policy.

## SSE

```go
import "github.com/boracomet/go-irmik/irmik/sse"

app.Engine.GET("/events", sse.Handler(sse.Options{
    Heartbeat: 15 * time.Second,
}, func(s *sse.Stream) error {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-s.Done(): // client disconnect or s.Close()
            return nil
        case t := <-ticker.C:
            _ = s.Event("tick", gin.H{"time": t.UTC().Format(time.RFC3339)})
        }
    }
}))
```

Helpers: `s.Write(Event{…})`, `s.Data(…)`, `s.Event(name, …)`, `s.Comment("ping")`. Non-string data is JSON-encoded. Multi-line data becomes multiple `data:` lines.

### Fan-out Hub

```go
hub := sse.NewHub()
go produce(hub) // hub.Broadcast(sse.Event{Data: "…"}})

app.Engine.GET("/stream", sse.Handler(sse.Options{}, func(s *sse.Stream) error {
    hub.Attach(s) // blocks until disconnect
    return nil
}))
```

## WebSocket

```go
import "github.com/boracomet/go-irmik/irmik/ws"

opts := ws.Options{AllowedOrigins: cfg.Realtime.AllowedOrigins, Development: cfg.IsDev()}

// Echo
app.Engine.GET("/ws/echo", ws.Handler(opts, func(c *ws.Conn) error {
    for {
        mt, msg, err := c.ReadMessage()
        if err != nil {
            return err
        }
        if err := c.WriteMessage(mt, msg); err != nil {
            return err
        }
    }
}))

// Chat hub with rooms (?room=lobby)
hub := ws.NewHub(opts)
hub.Start()
app.Engine.GET("/ws/chat", hub.ServeHTTP)
// hub.Broadcast([]byte("announce"))
// hub.BroadcastRoom("lobby", []byte("hi"))
```

Clients in a room rebroadcast inbound text to that room; clients with no room broadcast globally. The write pump sends WebSocket pings; graceful close uses a close control frame.

## Example

See [`examples/realtime`](../examples/realtime): SSE clock + broadcast stream, WS echo and chat room.
