package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
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

type Hub struct {
	mu       sync.RWMutex
	rooms    map[string]map[*Client]struct{}
	redis    *redis.Client
	upgrader websocket.Upgrader
	uploads  string
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	user string
	room string
}

type InboundMessage struct {
	Content      string `json:"content"`
	Type         string `json:"type"`
	MessageID    string `json:"message_id"`
	Meta         string `json:"meta"`
	Ephemeral    string `json:"ephemeral"` // after_read | 24h
	ReplyTo      string `json:"reply_to"`
	ReplyContent string `json:"reply_content"`
	ReplyUser    string `json:"reply_user"`
}

type OutboundMessage struct {
	ID           string `json:"id,omitempty"`
	User         string `json:"user"`
	Room         string `json:"room"`
	Content      string `json:"content"`
	Type         string `json:"type"`
	Timestamp    int64  `json:"ts"`
	Meta         string `json:"meta,omitempty"`
	MessageID    string `json:"message_id,omitempty"`
	Ephemeral    string `json:"ephemeral,omitempty"`
	ReplyTo      string `json:"reply_to,omitempty"`
	ReplyContent string `json:"reply_content,omitempty"`
	ReplyUser    string `json:"reply_user,omitempty"`
	RetryAfter   int    `json:"retry_after,omitempty"`
}

func NewHub(rdb *redis.Client, uploads string) *Hub {
	return &Hub{
		rooms:   make(map[string]map[*Client]struct{}),
		redis:   rdb,
		uploads: uploads,
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
		"type":   "presence",
		"room":   room,
		"user":   "system",
		"online": members,
		"ts":     time.Now().UnixMilli(),
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

	c.conn.SetReadLimit(8192)
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

		switch msgType {
		case "typing":
			payload, _ := json.Marshal(OutboundMessage{
				User:      c.user,
				Room:      c.room,
				Type:      "typing",
				Timestamp: time.Now().UnixMilli(),
			})
			_ = c.hub.redis.Publish(context.Background(), "chat:room:"+c.room, payload).Err()
			c.hub.setFocus(context.Background(), c.user, "typing", c.room)
			continue

		case "focus":
			st := strings.ToLower(strings.TrimSpace(msg.Content))
			if st == "" {
				st = "here"
			}
			c.hub.setFocus(context.Background(), c.user, st, c.room)
			continue

		case "read":
			mid := strings.TrimSpace(msg.MessageID)
			if mid == "" {
				continue
			}
			ctx := context.Background()
			_ = c.hub.redis.HSet(ctx, "read:"+c.room, c.user, mid).Err()
			payload, _ := json.Marshal(OutboundMessage{
				User:      c.user,
				Room:      c.room,
				Type:      "read",
				MessageID: mid,
				Timestamp: time.Now().UnixMilli(),
			})
			_ = c.hub.redis.Publish(ctx, "chat:room:"+c.room, payload).Err()
			c.hub.maybeExpireAfterRead(ctx, c.user, c.room, mid)
			continue

		case "reaction":
			mid := strings.TrimSpace(msg.MessageID)
			emoji := strings.TrimSpace(msg.Content)
			if mid == "" || emoji == "" || len([]rune(emoji)) > 8 {
				continue
			}
			ctx := context.Background()
			key := "reactions:" + c.room + ":" + mid
			_ = c.hub.redis.SAdd(ctx, key+":"+emoji, c.user).Err()
			_ = c.hub.redis.Expire(ctx, key+":"+emoji, 7*24*time.Hour).Err()
			payload, _ := json.Marshal(map[string]interface{}{
				"type":       "reaction",
				"user":       c.user,
				"room":       c.room,
				"message_id": mid,
				"content":    emoji,
				"ts":         time.Now().UnixMilli(),
			})
			_ = c.hub.redis.Publish(ctx, "chat:room:"+c.room, payload).Err()
			continue

		case "edit":
			c.hub.handleEditMessage(context.Background(), c.user, c.room, strings.TrimSpace(msg.MessageID), msg.Content)
			continue

		case "unsend":
			c.hub.handleUnsendMessage(context.Background(), c.user, c.room, strings.TrimSpace(msg.MessageID))
			continue
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if len(content) > 2000 {
			content = content[:2000]
		}
		if msgType != "chat" && msgType != "image" && msgType != "file" && msgType != "gif" && msgType != "sticker" && msgType != "album" {
			msgType = "chat"
		}

		// Drop outbound DMs / skip if peer blocked (still allow public rooms; client also filters)
		if strings.HasPrefix(c.room, "dm:") && c.hub.blockedEither(context.Background(), c.user, peerFromDM(c.room, c.user)) {
			continue
		}

		ephemeral := strings.ToLower(strings.TrimSpace(msg.Ephemeral))
		if ephemeral != "after_read" && ephemeral != "24h" {
			ephemeral = ""
		}

		fields := map[string]interface{}{
			"user":    c.user,
			"room":    c.room,
			"content": content,
			"type":    msgType,
			"meta":    msg.Meta,
			"ts":      time.Now().UnixMilli(),
		}
		if ephemeral != "" {
			fields["ephemeral"] = ephemeral
		}
		if mid := strings.TrimSpace(msg.ReplyTo); mid != "" {
			fields["reply_to"] = mid
			fields["reply_content"] = strings.TrimSpace(msg.ReplyContent)
			fields["reply_user"] = strings.TrimSpace(msg.ReplyUser)
		}
		c.hub.trackMedia(context.Background(), c.user, c.room, msgType, content, msg.Meta)
		c.hub.setFocus(context.Background(), c.user, "here", c.room)
		if err := c.hub.redis.XAdd(context.Background(), &redis.XAddArgs{
			Stream: inboundStream,
			Values: fields,
		}).Err(); err != nil {
			log.Printf("xadd error: %v", err)
		}
	}
}

func peerFromDM(room, me string) string {
	parts := strings.Split(room, ":")
	if len(parts) != 3 {
		return ""
	}
	if parts[1] == me {
		return parts[2]
	}
	return parts[1]
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
	pubsub := h.redis.PSubscribe(ctx, "chat:room:*", "chat:user:*", "chat:focus")
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
			if strings.HasPrefix(msg.Channel, "chat:room:") {
				room := strings.TrimPrefix(msg.Channel, "chat:room:")
				h.broadcastLocal(room, []byte(msg.Payload))
				var m map[string]interface{}
				if json.Unmarshal([]byte(msg.Payload), &m) == nil {
					t := str(m["type"])
					if t == "chat" || t == "image" || t == "file" || t == "gif" || t == "sticker" || t == "album" {
						h.bumpUnread(ctx, room, str(m["user"]))
					}
				}
			} else if strings.HasPrefix(msg.Channel, "chat:user:") {
				user := strings.TrimPrefix(msg.Channel, "chat:user:")
				h.deliverToUser(user, []byte(msg.Payload))
			} else if msg.Channel == "chat:focus" {
				h.broadcastAll([]byte(msg.Payload))
			}
		}
	}
}

func (h *Hub) deliverToUser(user string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, clients := range h.rooms {
		for c := range clients {
			if c.user == user {
				select {
				case c.send <- payload:
				default:
				}
			}
		}
	}
}

func (h *Hub) broadcastAll(payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := map[*Client]struct{}{}
	for _, clients := range h.rooms {
		for c := range clients {
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			select {
			case c.send <- payload:
			default:
			}
		}
	}
}

func (h *Hub) maybeExpireAfterRead(ctx context.Context, reader, room, mid string) {
	idx, m, err := h.findHistoryIndex(ctx, room, mid)
	if err != nil || m == nil {
		return
	}
	if str(m["ephemeral"]) != "after_read" {
		return
	}
	author := str(m["user"])
	if author == "" || author == reader {
		return
	}
	// slight delay so reader sees it briefly
	go func() {
		time.Sleep(800 * time.Millisecond)
		h.handleUnsendMessage(context.Background(), author, room, mid)
		_ = idx
	}()
}

func (h *Hub) canJoinRoom(ctx context.Context, user, room string) bool {
	if strings.HasPrefix(room, "dm:") {
		parts := strings.Split(room, ":")
		if len(parts) != 3 {
			return false
		}
		return user == parts[1] || user == parts[2]
	}
	return true
}

func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, ok := h.userFromToken(r.Context(), token)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	room := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("room")))
	if room == "" {
		http.Error(w, "room required", http.StatusBadRequest)
		return
	}
	if len(room) > 64 || !validRoom(room) {
		http.Error(w, "invalid room", http.StatusBadRequest)
		return
	}
	if !h.canJoinRoom(r.Context(), user, room) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if strings.HasPrefix(room, "dm:") {
		peer := peerFromDM(room, user)
		if peer != "" && h.blockedEither(r.Context(), user, peer) {
			http.Error(w, "blocked", http.StatusForbidden)
			return
		}
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
	h.setFocus(r.Context(), user, "here", room)
	h.clearUnread(r.Context(), user, room)

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
	return len(s) > 0
}

func validRoom(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == ':' {
			continue
		}
		return false
	}
	return len(s) > 0
}
