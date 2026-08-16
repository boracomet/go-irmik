// Package ws provides WebSocket upgrade helpers and a room-aware Hub for Gin.
package ws

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	defaultWriteWait  = 10 * time.Second
	defaultPongWait   = 60 * time.Second
	defaultPingPeriod = 54 * time.Second // < pongWait
	defaultSendBuf    = 64
	maxMessageSize    = 512 * 1024
)

// Options configures upgrade and connection pumps.
type Options struct {
	// AllowedOrigins lists exact Origin values (scheme+host[+port]).
	// Empty rejects browser upgrades unless Development is true.
	AllowedOrigins []string
	// Development permits any origin when AllowedOrigins is empty. Set this from
	// config.Config.IsDev() for local demos; production remains fail-closed.
	Development bool
	// CheckOrigin overrides AllowedOrigins when set.
	CheckOrigin func(r *http.Request) bool
	// ReadBufferSize / WriteBufferSize passed to the upgrader (0 = library default).
	ReadBufferSize  int
	WriteBufferSize int
	// SendBuffer is the outbound message channel size (default 64).
	SendBuffer int
	// PingPeriod / PongWait / WriteWait tune keepalive (zero → defaults).
	PingPeriod time.Duration
	PongWait   time.Duration
	WriteWait  time.Duration
	// MaxMessageSize limits inbound frames (default 512KiB).
	MaxMessageSize int64
}

// Conn is an upgraded WebSocket connection with helpers.
type Conn struct {
	*websocket.Conn
	opts Options
}

// Upgrade upgrades the Gin request to a WebSocket Conn.
func Upgrade(c *gin.Context, opts Options) (*Conn, error) {
	up := websocket.Upgrader{
		ReadBufferSize:  opts.ReadBufferSize,
		WriteBufferSize: opts.WriteBufferSize,
		CheckOrigin:     checkOriginFunc(opts),
	}
	raw, err := up.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, err
	}
	return &Conn{Conn: raw, opts: opts}, nil
}

// Handler upgrades and runs fn; the connection is closed when fn returns.
func Handler(opts Options, fn func(*Conn) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := Upgrade(c, opts)
		if err != nil {
			return // Upgrade already wrote the error response when possible
		}
		defer conn.CloseGracefully(websocket.CloseNormalClosure, "bye")
		_ = fn(conn)
	}
}

// CloseGracefully sends a close frame then closes the underlying connection.
func (c *Conn) CloseGracefully(code int, reason string) {
	deadline := time.Now().Add(resolveWriteWait(c.opts))
	_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), deadline)
	_ = c.Close()
}

func checkOriginFunc(opts Options) func(*http.Request) bool {
	if opts.CheckOrigin != nil {
		return opts.CheckOrigin
	}
	allowed := make(map[string]struct{}, len(opts.AllowedOrigins))
	for _, o := range opts.AllowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return func(*http.Request) bool { return opts.Development }
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		if _, ok := allowed[origin]; ok {
			return true
		}
		// Also allow matching Host when Origin is same-site without port quirks.
		if u, err := url.Parse(origin); err == nil {
			if u.Host == r.Host {
				return true
			}
		}
		return false
	}
}

func resolveWriteWait(o Options) time.Duration {
	if o.WriteWait > 0 {
		return o.WriteWait
	}
	return defaultWriteWait
}

func resolvePongWait(o Options) time.Duration {
	if o.PongWait > 0 {
		return o.PongWait
	}
	return defaultPongWait
}

func resolvePingPeriod(o Options) time.Duration {
	if o.PingPeriod > 0 {
		return o.PingPeriod
	}
	return defaultPingPeriod
}

func resolveMaxMsg(o Options) int64 {
	if o.MaxMessageSize > 0 {
		return o.MaxMessageSize
	}
	return maxMessageSize
}

func resolveSendBuf(o Options) int {
	if o.SendBuffer > 0 {
		return o.SendBuffer
	}
	return defaultSendBuf
}

// Hub manages clients and optional named rooms.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	rooms   map[string]map[*Client]struct{}
	opts    Options

	register   chan *Client
	unregister chan *Client
	broadcast  chan Envelope

	done chan struct{}
	once sync.Once
}

// Envelope is a hub message, optionally scoped to a room.
type Envelope struct {
	Room string // empty → all clients
	Data []byte
}

// NewHub creates a Hub. Call Run in a goroutine (or use Start).
func NewHub(opts Options) *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		rooms:      make(map[string]map[*Client]struct{}),
		opts:       opts,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Envelope, 256),
		done:       make(chan struct{}),
	}
}

// Start launches the hub event loop in a background goroutine.
func (h *Hub) Start() {
	go h.Run()
}

// Run processes register/unregister/broadcast until Close.
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			return
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unregister:
			h.removeClient(c)
		case env := <-h.broadcast:
			h.dispatch(env)
		}
	}
}

// Close stops the hub loop. Clients should be closed by their handlers.
func (h *Hub) Close() {
	h.once.Do(func() { close(h.done) })
}

// Broadcast sends data to every connected client.
func (h *Hub) Broadcast(data []byte) {
	select {
	case h.broadcast <- Envelope{Data: data}:
	case <-h.done:
	}
}

// BroadcastRoom sends data to clients joined to room.
func (h *Hub) BroadcastRoom(room string, data []byte) {
	select {
	case h.broadcast <- Envelope{Room: room, Data: data}:
	case <-h.done:
	}
}

// Len returns the number of connected clients.
func (h *Hub) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeHTTP upgrades the Gin request, registers a Client, and runs pumps.
// Optional room query (?room=) joins that room on connect.
func (h *Hub) ServeHTTP(c *gin.Context) {
	conn, err := Upgrade(c, h.opts)
	if err != nil {
		return
	}
	client := newClient(h, conn)
	if room := strings.TrimSpace(c.Query("room")); room != "" {
		client.Join(room)
	}
	h.register <- client
	go client.writePump()
	client.readPump()
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; !ok {
		return
	}
	delete(h.clients, c)
	for room := range c.rooms {
		if m, ok := h.rooms[room]; ok {
			delete(m, c)
			if len(m) == 0 {
				delete(h.rooms, room)
			}
		}
	}
	close(c.send)
	_ = c.conn.Close()
}

func (h *Hub) dispatch(env Envelope) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	targets := h.clients
	if env.Room != "" {
		targets = nil
		if m, ok := h.rooms[env.Room]; ok {
			targets = make(map[*Client]struct{}, len(m))
			for c := range m {
				targets[c] = struct{}{}
			}
		}
	}
	for c := range targets {
		select {
		case c.send <- env.Data:
		default:
			// Slow consumer: drop this message rather than block the hub.
		}
	}
}

func (h *Hub) joinRoom(c *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m, ok := h.rooms[room]
	if !ok {
		m = make(map[*Client]struct{})
		h.rooms[room] = m
	}
	m[c] = struct{}{}
	c.rooms[room] = struct{}{}
}

func (h *Hub) leaveRoom(c *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.rooms[room]; ok {
		delete(m, c)
		if len(m) == 0 {
			delete(h.rooms, room)
		}
	}
	delete(c.rooms, room)
}

// Client is a hub-managed WebSocket peer.
type Client struct {
	hub   *Hub
	conn  *Conn
	send  chan []byte
	rooms map[string]struct{} // guarded by hub.mu
}

func newClient(h *Hub, conn *Conn) *Client {
	return &Client{
		hub:   h,
		conn:  conn,
		send:  make(chan []byte, resolveSendBuf(h.opts)),
		rooms: make(map[string]struct{}),
	}
}

// Join adds the client to a named room.
func (c *Client) Join(room string) {
	room = strings.TrimSpace(room)
	if room == "" {
		return
	}
	c.hub.joinRoom(c, room)
}

// Leave removes the client from a named room.
func (c *Client) Leave(room string) {
	c.hub.leaveRoom(c, room)
}

// Send queues an outbound message (non-blocking; drops if full).
func (c *Client) Send(data []byte) {
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
	}()
	opts := c.hub.opts
	c.conn.SetReadLimit(resolveMaxMsg(opts))
	_ = c.conn.SetReadDeadline(time.Now().Add(resolvePongWait(opts)))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(resolvePongWait(opts)))
	})
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// Default chat behaviour: rebroadcast to client's rooms, or globally.
		rooms := c.roomNames()
		if len(rooms) == 0 {
			c.hub.Broadcast(msg)
		} else {
			for _, r := range rooms {
				c.hub.BroadcastRoom(r, msg)
			}
		}
	}
}

func (c *Client) roomNames() []string {
	c.hub.mu.RLock()
	defer c.hub.mu.RUnlock()
	rooms := make([]string, 0, len(c.rooms))
	for r := range c.rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

func (c *Client) writePump() {
	opts := c.hub.opts
	ticker := time.NewTicker(resolvePingPeriod(opts))
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(resolveWriteWait(opts)))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(resolveWriteWait(opts)))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
