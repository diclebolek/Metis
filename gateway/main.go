package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	inboundStream = "messages:inbound"
	roomsSet      = "chat:rooms"
	presenceTTL   = 60 * time.Second
)

type Config struct {
	HTTPAddr  string
	RedisAddr string
}

func loadConfig() Config {
	return Config{
		HTTPAddr:  envOr("HTTP_ADDR", ":8080"),
		RedisAddr: envOr("REDIS_ADDR", "127.0.0.1:6379"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type Hub struct {
	mu       sync.RWMutex
	rooms    map[string]map[*Client]struct{}
	redis    *redis.Client
	upgrader websocket.Upgrader
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	user string
	room string
}

type InboundMessage struct {
	Content string `json:"content"`
	Type    string `json:"type"`
}

type OutboundMessage struct {
	ID        string `json:"id,omitempty"`
	User      string `json:"user"`
	Room      string `json:"room"`
	Content   string `json:"content"`
	Type      string `json:"type"`
	Timestamp int64  `json:"ts"`
}

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		rooms: make(map[string]map[*Client]struct{}),
		redis: rdb,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Hub) join(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[c.room] == nil {
		h.rooms[c.room] = make(map[*Client]struct{})
	}
	h.rooms[c.room][c] = struct{}{}
}

func (h *Hub) leave(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.rooms[c.room]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.rooms, c.room)
		}
	}
	close(c.send)
}

func (h *Hub) broadcastLocal(room string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[room] {
		select {
		case c.send <- payload:
		default:
			go c.conn.Close()
		}
	}
}

func (h *Hub) setPresence(ctx context.Context, room, user string) {
	key := "presence:" + room
	_ = h.redis.SAdd(ctx, key, user).Err()
	_ = h.redis.Expire(ctx, key, presenceTTL).Err()
	_ = h.redis.SAdd(ctx, roomsSet, room).Err()
}

func (h *Hub) clearPresence(ctx context.Context, room, user string) {
	_ = h.redis.SRem(ctx, "presence:"+room, user).Err()
}

func (h *Hub) publishPresence(ctx context.Context, room string) {
	members, err := h.redis.SMembers(ctx, "presence:"+room).Result()
	if err != nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"type":    "presence",
		"room":    room,
		"user":    "system",
		"content": "",
		"online":  members,
		"ts":      time.Now().UnixMilli(),
	})
	_ = h.redis.Publish(ctx, "chat:room:"+room, payload).Err()
}

func (c *Client) readPump() {
	defer func() {
		ctx := context.Background()
		c.hub.clearPresence(ctx, c.room, c.user)
		c.hub.leave(c)
		c.conn.Close()
		sys, _ := json.Marshal(OutboundMessage{
			User:      "system",
			Room:      c.room,
			Content:   c.user + " left",
			Type:      "system",
			Timestamp: time.Now().UnixMilli(),
		})
		_ = c.hub.redis.Publish(ctx, "chat:room:"+c.room, sys).Err()
		c.hub.publishPresence(ctx, c.room)
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.hub.setPresence(context.Background(), c.room, c.user)
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg InboundMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		msgType := strings.TrimSpace(msg.Type)
		if msgType == "" {
			msgType = "chat"
		}

		// Typing events bypass the worker for lower latency.
		if msgType == "typing" {
			payload, _ := json.Marshal(OutboundMessage{
				User:      c.user,
				Room:      c.room,
				Content:   "",
				Type:      "typing",
				Timestamp: time.Now().UnixMilli(),
			})
			_ = c.hub.redis.Publish(context.Background(), "chat:room:"+c.room, payload).Err()
			continue
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if len(content) > 1000 {
			content = content[:1000]
		}

		fields := map[string]interface{}{
			"user":    c.user,
			"room":    c.room,
			"content": content,
			"type":    "chat",
			"ts":      time.Now().UnixMilli(),
		}
		if err := c.hub.redis.XAdd(context.Background(), &redis.XAddArgs{
			Stream: inboundStream,
			Values: fields,
		}).Err(); err != nil {
			log.Printf("xadd error: %v", err)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) subscribeRedis(ctx context.Context) {
	pubsub := h.redis.PSubscribe(ctx, "chat:room:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			room := strings.TrimPrefix(msg.Channel, "chat:room:")
			h.broadcastLocal(room, []byte(msg.Payload))
		}
	}
}

func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	room := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("room")))
	if user == "" || room == "" {
		http.Error(w, "user and room required", http.StatusBadRequest)
		return
	}
	if len(user) > 32 || len(room) > 32 {
		http.Error(w, "user/room too long", http.StatusBadRequest)
		return
	}
	if !validName(user) || !validName(room) {
		http.Error(w, "invalid user or room name", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 64),
		user: user,
		room: room,
	}
	h.join(client)
	h.setPresence(r.Context(), room, user)

	joinMsg, _ := json.Marshal(OutboundMessage{
		User:      "system",
		Room:      room,
		Content:   user + " joined",
		Type:      "system",
		Timestamp: time.Now().UnixMilli(),
	})
	_ = h.redis.Publish(r.Context(), "chat:room:"+room, joinMsg).Err()
	h.publishPresence(r.Context(), room)

	go client.writePump()
	go client.readPump()
}

func validName(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func (h *Hub) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := h.redis.Ping(r.Context()).Err(); err != nil {
		http.Error(w, `{"ok":false}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true,"service":"gateway"}`))
}

func (h *Hub) handleListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.redis.SMembers(r.Context(), roomsSet).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type roomInfo struct {
		Name    string   `json:"name"`
		Online  []string `json:"online"`
		OnlineN int      `json:"online_count"`
	}
	out := make([]roomInfo, 0, len(rooms))
	for _, name := range rooms {
		members, _ := h.redis.SMembers(r.Context(), "presence:"+name).Result()
		out = append(out, roomInfo{Name: name, Online: members, OnlineN: len(members)})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Hub) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(strings.ToLower(body.Name))
	if name == "" || len(name) > 32 || !validName(name) {
		http.Error(w, "invalid room name", http.StatusBadRequest)
		return
	}
	if err := h.redis.SAdd(r.Context(), roomsSet, name).Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": name})
}

func (h *Hub) handleRoomSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "room required", http.StatusBadRequest)
		return
	}
	room := strings.ToLower(parts[0])
	if len(parts) == 1 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	switch parts[1] {
	case "messages":
		h.handleMessages(w, r, room)
	case "presence":
		h.handlePresence(w, r, room)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *Hub) handleMessages(w http.ResponseWriter, r *http.Request, room string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := h.redis.LRange(r.Context(), "history:"+room, 0, 49).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	msgs := make([]json.RawMessage, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		msgs = append(msgs, json.RawMessage(items[i]))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}

func (h *Hub) handlePresence(w http.ResponseWriter, r *http.Request, room string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	members, err := h.redis.SMembers(r.Context(), "presence:"+room).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"room":   room,
		"online": members,
		"count":  len(members),
	})
}

func seedRooms(ctx context.Context, rdb *redis.Client) {
	for _, name := range []string{"general", "random", "dev"} {
		_ = rdb.SAdd(ctx, roomsSet, name).Err()
	}
}

func main() {
	cfg := loadConfig()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}
	seedRooms(ctx, rdb)

	hub := NewHub(rdb)
	go hub.subscribeRedis(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", hub.handleHealth)
	mux.HandleFunc("/api/rooms", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			hub.handleListRooms(w, r)
		case http.MethodPost:
			hub.handleCreateRoom(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/rooms/", hub.handleRoomSubroutes)
	mux.HandleFunc("/ws", hub.handleWS)
	mux.Handle("/", http.FileServer(http.Dir("../web")))

	log.Printf("gateway listening on %s (redis %s)", cfg.HTTPAddr, cfg.RedisAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, mux))
}
