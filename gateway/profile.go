package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (h *Hub) blockedEither(ctx context.Context, user, peer string) bool {
	a, _ := h.redis.SIsMember(ctx, "user:"+user+":blocked", peer).Result()
	b, _ := h.redis.SIsMember(ctx, "user:"+peer+":blocked", user).Result()
	return a || b
}

func (h *Hub) handleUserRoutes(w http.ResponseWriter, r *http.Request) {
	me, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/users/"), "/")
	if path == "" {
		h.handleUsers(w, r)
		return
	}
	parts := strings.Split(path, "/")
	name := strings.ToLower(parts[0])
	if !validName(name) {
		http.Error(w, "invalid user", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.handleUserProfile(w, r, me, name)
		return
	}

	switch parts[1] {
	case "media":
		h.handleUserMedia(w, r, me, name)
	case "block":
		h.handleUserBlock(w, r, me, name)
	case "mute":
		h.handleUserMute(w, r, me, name)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *Hub) handleUserProfile(w http.ResponseWriter, r *http.Request, me, name string) {
	exists, err := h.redis.Exists(r.Context(), "user:"+name).Result()
	if err != nil || exists == 0 {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	blocked, _ := h.redis.SIsMember(r.Context(), "user:"+me+":blocked", name).Result()
	muted, _ := h.redis.SIsMember(r.Context(), "user:"+me+":muted", name).Result()
	mediaN, _ := h.redis.LLen(r.Context(), "media:"+name).Result()
	writeJSON(w, map[string]interface{}{
		"username":    name,
		"blocked":     blocked,
		"muted":       muted,
		"media_count": mediaN,
		"self":        me == name,
	})
}

func (h *Hub) handleUserMedia(w http.ResponseWriter, r *http.Request, me, name string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.blockedEither(r.Context(), me, name) && me != name {
		http.Error(w, "blocked", http.StatusForbidden)
		return
	}
	items, err := h.redis.LRange(r.Context(), "media:"+name, 0, 49).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]json.RawMessage, 0, len(items))
	for _, it := range items {
		out = append(out, json.RawMessage(it))
	}
	writeJSON(w, out)
}

func (h *Hub) handleUserBlock(w http.ResponseWriter, r *http.Request, me, name string) {
	if me == name {
		http.Error(w, "cannot block yourself", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPost:
		_ = h.redis.SAdd(r.Context(), "user:"+me+":blocked", name).Err()
		writeJSON(w, map[string]interface{}{"blocked": true, "user": name})
	case http.MethodDelete:
		_ = h.redis.SRem(r.Context(), "user:"+me+":blocked", name).Err()
		writeJSON(w, map[string]interface{}{"blocked": false, "user": name})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Hub) handleUserMute(w http.ResponseWriter, r *http.Request, me, name string) {
	if me == name {
		http.Error(w, "cannot mute yourself", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPost:
		_ = h.redis.SAdd(r.Context(), "user:"+me+":muted", name).Err()
		writeJSON(w, map[string]interface{}{"muted": true, "user": name})
	case http.MethodDelete:
		_ = h.redis.SRem(r.Context(), "user:"+me+":muted", name).Err()
		writeJSON(w, map[string]interface{}{"muted": false, "user": name})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Hub) trackMedia(ctx context.Context, user, room, msgType, url, meta string) {
	if msgType != "image" && msgType != "gif" {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"url":  url,
		"type": msgType,
		"room": room,
		"user": user,
		"meta": meta,
		"ts":   time.Now().UnixMilli(),
	})
	raw := string(payload)
	userKey := "media:" + user
	_ = h.redis.LPush(ctx, userKey, raw).Err()
	_ = h.redis.LTrim(ctx, userKey, 0, 49).Err()
	if room != "" {
		roomKey := "media:room:" + room
		_ = h.redis.LPush(ctx, roomKey, raw).Err()
		_ = h.redis.LTrim(ctx, roomKey, 0, 99).Err()
	}
}
