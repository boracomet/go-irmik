# Realtime example

Demo: SSE clock + hub stream, WebSocket echo + chat room.

```bash
cd examples/realtime
go run .
# http://127.0.0.1:8080
```

Open `/` in a browser, or:

```bash
# SSE clock (Ctrl-C to stop)
curl -N http://127.0.0.1:8080/sse/clock

# WebSocket echo (needs websocat or similar)
websocat ws://127.0.0.1:8080/ws/echo
websocat 'ws://127.0.0.1:8080/ws/chat?room=lobby'
```

`server.writeTimeout` / `readTimeout` are set to `0` in `irmik.yaml` so streams stay open.
