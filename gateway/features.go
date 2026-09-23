package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func secondsUntilMidnight() int {
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return int(tomorrow.Sub(now).Seconds())
}

func ttlFromOption(opt string) (int, time.Time, bool) {
	now := time.Now()
	switch strings.ToLower(strings.TrimSpace(opt)) {
	case "2h", "2hour", "2hours":
		return 2 * 3600, now.Add(2 * time.Hour), true
	case "24h", "24hour", "24hours":
		return 24 * 3600, now.Add(24 * time.Hour), true
	case "midnight", "gece", "tonight":
		sec := secondsUntilMidnight()
		if sec < 60 {
			sec = 60
		}
		return sec, now.Add(time.Duration(sec) * time.Second), true
	default:
		return 0, time.Time{}, false
	}
}

func (h *Hub) applyRoomTTL(ctx context.Context, room string, ttlSec int) {
	if ttlSec <= 0 {
		return
	}
	d := time.Duration(ttlSec) * time.Second
	_ = h.redis.Expire(ctx, "history:"+room, d).Err()
	_ = h.redis.Expire(ctx, "presence:"+room, d).Err()
	_ = h.redis.Expire(ctx, "roommeta:"+room, d).Err()
	_ = h.redis.Expire(ctx, "pins:"+room, d).Err()
	_ = h.redis.Expire(ctx, "read:"+room, d).Err()
}

func (h *Hub) sweepExpiredRooms(ctx context.Context) {
	rooms, _ := h.redis.SMembers(ctx, roomsSet).Result()
	for _, room := range rooms {
		if strings.HasPrefix(room, "dm:") {
			continue
		}
		n, err := h.redis.Exists(ctx, "roommeta:"+room).Result()
		if err != nil {
			continue
		}
		kind, _ := h.redis.HGet(ctx, "roommeta:"+room, "kind").Result()
		if kind == "timed" && n == 0 {
			_ = h.redis.SRem(ctx, roomsSet, room).Err()
			continue
		}
		// also drop if meta says expired
		expStr, _ := h.redis.HGet(ctx, "roommeta:"+room, "expires_at").Result()
		if expStr != "" {
			if exp, err := strconv.ParseInt(expStr, 10, 64); err == nil && time.Now().Unix() > exp {
				_ = h.redis.SRem(ctx, roomsSet, room).Err()
				_ = h.redis.Del(ctx, "history:"+room, "presence:"+room, "roommeta:"+room, "pins:"+room).Err()
			}
		}
	}
}

func (h *Hub) startFeatureLoops(ctx context.Context) {
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				h.sweepExpiredRooms(ctx)
				h.sweepEphemeral(ctx)
			}
		}
	}()
}

func (h *Hub) sweepEphemeral(ctx context.Context) {
	rooms, _ := h.redis.SMembers(ctx, roomsSet).Result()
	now := time.Now().UnixMilli()
	for _, room := range rooms {
		items, err := h.redis.LRange(ctx, "history:"+room, 0, -1).Result()
		if err != nil {
			continue
		}
		for i, raw := range items {
			var m map[string]interface{}
			if json.Unmarshal([]byte(raw), &m) != nil {
				continue
			}
			if str(m["ephemeral"]) != "24h" {
				continue
			}
			exp := numI64(m["expire_at"])
			if exp <= 0 || now <= exp {
				continue
			}
			mid := str(m["id"])
			m["type"] = "unsent"
			m["content"] = ""
			m["ephemeral"] = ""
			b, _ := json.Marshal(m)
			_ = h.redis.LSet(ctx, "history:"+room, int64(i), string(b)).Err()
			payload, _ := json.Marshal(map[string]interface{}{
				"type": "unsend", "message_id": mid, "user": "system", "room": room,
			})
			_ = h.redis.Publish(ctx, "chat:room:"+room, payload).Err()
		}
	}
}

func (h *Hub) setFocus(ctx context.Context, user, status, room string) {
	if status == "" {
		status = "here"
	}
	_ = h.redis.HSet(ctx, "focus:"+user, map[string]interface{}{
		"status": status,
		"room":   room,
		"ts":     time.Now().UnixMilli(),
	}).Err()
	_ = h.redis.Expire(ctx, "focus:"+user, 2*time.Minute).Err()
	payload, _ := json.Marshal(map[string]interface{}{
		"type":   "focus",
		"user":   user,
		"status": status,
		"room":   room,
		"ts":     time.Now().UnixMilli(),
	})
	_ = h.redis.Publish(ctx, "chat:focus", payload).Err()
	if room != "" {
		_ = h.redis.Publish(ctx, "chat:room:"+room, payload).Err()
	}
}

func (h *Hub) handleFocus(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Status string `json:"status"`
		Room   string `json:"room"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	st := strings.ToLower(strings.TrimSpace(body.Status))
	switch st {
	case "here", "typing", "busy", "away":
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	h.setFocus(r.Context(), user, st, strings.TrimSpace(body.Room))
	writeJSON(w, map[string]string{"status": st, "room": body.Room})
}

func (h *Hub) handlePin(w http.ResponseWriter, r *http.Request, room string) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	key := "pins:" + room
	switch r.Method {
	case http.MethodGet:
		items, _ := h.redis.LRange(r.Context(), key, 0, 19).Result()
		out := make([]json.RawMessage, 0, len(items))
		for _, it := range items {
			out = append(out, json.RawMessage(it))
		}
		writeJSON(w, out)
	case http.MethodPost:
		var body struct {
			MessageID string `json:"message_id"`
			Content   string `json:"content"`
			User      string `json:"user"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MessageID == "" {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"id": body.MessageID, "content": body.Content, "user": body.User, "by": user, "ts": time.Now().UnixMilli(),
		})
		_ = h.redis.LPush(r.Context(), key, string(payload)).Err()
		_ = h.redis.LTrim(r.Context(), key, 0, 19).Err()
		pub, _ := json.Marshal(map[string]interface{}{"type": "pin", "room": room, "payload": json.RawMessage(payload)})
		_ = h.redis.Publish(r.Context(), "chat:room:"+room, pub).Err()
		writeJSON(w, map[string]string{"ok": "1"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Hub) bumpUnread(ctx context.Context, room, exceptUser string) {
	// naive: all users get +1 except sender — use registered users set
	users, _ := h.redis.SMembers(ctx, usersSet).Result()
	for _, u := range users {
		if u == exceptUser {
			continue
		}
		// skip if currently focused in this room
		fr, _ := h.redis.HGet(ctx, "focus:"+u, "room").Result()
		if fr == room {
			continue
		}
		_ = h.redis.HIncrBy(ctx, "unread:"+u, room, 1).Err()
	}
}

func (h *Hub) clearUnread(ctx context.Context, user, room string) {
	_ = h.redis.HDel(ctx, "unread:"+user, room).Err()
}

func (h *Hub) handleUnread(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	m, err := h.redis.HGetAll(r.Context(), "unread:"+user).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, m)
}

func (h *Hub) handleSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if len(q) < 2 {
		writeJSON(w, []interface{}{})
		return
	}
	rooms, _ := h.redis.SMembers(r.Context(), roomsSet).Result()
	type hit struct {
		Room    string `json:"room"`
		User    string `json:"user"`
		Content string `json:"content"`
		ID      string `json:"id"`
		TS      int64  `json:"ts"`
	}
	out := make([]hit, 0)
	for _, room := range rooms {
		if strings.HasPrefix(room, "dm:") {
			parts := strings.Split(room, ":")
			if len(parts) != 3 || (user != parts[1] && user != parts[2]) {
				continue
			}
		}
		items, _ := h.redis.LRange(r.Context(), "history:"+room, 0, 49).Result()
		for _, raw := range items {
			var m map[string]interface{}
			if json.Unmarshal([]byte(raw), &m) != nil {
				continue
			}
			content := str(m["content"])
			if !strings.Contains(strings.ToLower(content), q) {
				continue
			}
			var ts int64
			switch v := m["ts"].(type) {
			case float64:
				ts = int64(v)
			}
			out = append(out, hit{Room: room, User: str(m["user"]), Content: content, ID: str(m["id"]), TS: ts})
			if len(out) >= 40 {
				writeJSON(w, out)
				return
			}
		}
	}
	writeJSON(w, out)
}

func (h *Hub) handleInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Room string `json:"room"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		room := strings.TrimSpace(strings.ToLower(body.Room))
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		code := hex.EncodeToString(b)
		_ = h.redis.Set(r.Context(), "invite:"+code, room, 7*24*time.Hour).Err()
		_ = h.redis.SAdd(r.Context(), "room:admins:"+room, user).Err()
		writeJSON(w, map[string]string{"code": code, "room": room, "url": "/?invite=" + code})
	case http.MethodGet:
		code := r.URL.Query().Get("code")
		room, err := h.redis.Get(r.Context(), "invite:"+code).Result()
		if err != nil || room == "" {
			http.Error(w, "invalid invite", http.StatusNotFound)
			return
		}
		_ = h.redis.SAdd(r.Context(), roomsSet, room).Err()
		writeJSON(w, map[string]string{"room": room})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Hub) handleSignal(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body["from"] = user
	body["type"] = "call"
	raw, _ := json.Marshal(body)
	to, _ := body["to"].(string)
	if to == "" {
		http.Error(w, "to required", http.StatusBadRequest)
		return
	}
	_ = h.redis.Publish(r.Context(), "chat:user:"+to, string(raw)).Err()
	writeJSON(w, map[string]string{"ok": "1"})
}
