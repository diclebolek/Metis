package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (h *Hub) handleListRooms(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rooms, err := h.redis.SMembers(r.Context(), roomsSet).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	unread, _ := h.redis.HGetAll(r.Context(), "unread:"+user).Result()
	type roomInfo struct {
		Name      string   `json:"name"`
		Kind      string   `json:"kind"`
		Online    []string `json:"online"`
		OnlineN   int      `json:"online_count"`
		Peer      string   `json:"peer,omitempty"`
		ExpiresAt int64    `json:"expires_at,omitempty"`
		TTLLabel  string   `json:"ttl_label,omitempty"`
		Unread    int      `json:"unread,omitempty"`
	}
	out := make([]roomInfo, 0)
	for _, name := range rooms {
		uc := 0
		if s, ok := unread[name]; ok {
			uc, _ = strconv.Atoi(s)
		}
		if strings.HasPrefix(name, "dm:") {
			parts := strings.Split(name, ":")
			if len(parts) != 3 || (user != parts[1] && user != parts[2]) {
				continue
			}
			peer := parts[1]
			if peer == user {
				peer = parts[2]
			}
			members, _ := h.redis.SMembers(r.Context(), "presence:"+name).Result()
			out = append(out, roomInfo{Name: name, Kind: "dm", Online: members, OnlineN: len(members), Peer: peer, Unread: uc})
			continue
		}
		members, _ := h.redis.SMembers(r.Context(), "presence:"+name).Result()
		meta, _ := h.redis.HGetAll(r.Context(), "roommeta:"+name).Result()
		kind := "room"
		var exp int64
		ttlLabel := ""
		if meta["kind"] == "timed" {
			kind = "timed"
			exp, _ = strconv.ParseInt(meta["expires_at"], 10, 64)
			ttlLabel = meta["ttl"]
		} else if meta["kind"] == "event" {
			kind = "event"
			exp, _ = strconv.ParseInt(meta["expires_at"], 10, 64)
			ttlLabel = meta["ttl"]
		}
		out = append(out, roomInfo{
			Name: name, Kind: kind, Online: members, OnlineN: len(members),
			ExpiresAt: exp, TTLLabel: ttlLabel, Unread: uc,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	writeJSON(w, out)
}

func (h *Hub) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Name string `json:"name"`
		TTL  string `json:"ttl"`  // "", "2h", "midnight", "24h"
		Kind string `json:"kind"` // "room" | "timed" | "event"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(strings.ToLower(body.Name))
	if name == "" || len(name) > 32 || !validName(name) || strings.HasPrefix(name, "dm") {
		http.Error(w, "invalid room name", http.StatusBadRequest)
		return
	}
	kind := strings.ToLower(strings.TrimSpace(body.Kind))
	if kind == "" {
		if body.TTL != "" {
			kind = "timed"
		} else {
			kind = "room"
		}
	}
	if kind == "event" && body.TTL == "" {
		body.TTL = "24h"
	}
	ctx := r.Context()
	if err := h.redis.SAdd(ctx, roomsSet, name).Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.redis.SAdd(ctx, "room:admins:"+name, user).Err()

	resp := map[string]interface{}{"name": name, "kind": kind}
	if ttlSec, expires, okTTL := ttlFromOption(body.TTL); okTTL {
		_ = h.redis.HSet(ctx, "roommeta:"+name, map[string]interface{}{
			"kind":       kind,
			"ttl":        body.TTL,
			"expires_at": expires.Unix(),
			"created_by": user,
			"created_at": time.Now().Unix(),
		}).Err()
		h.applyRoomTTL(ctx, name, ttlSec)
		resp["expires_at"] = expires.Unix()
		resp["ttl"] = body.TTL
	} else if kind == "room" {
		_ = h.redis.HSet(ctx, "roommeta:"+name, map[string]interface{}{
			"kind": "room", "created_by": user,
		}).Err()
	}
	writeJSON(w, resp)
}

func (h *Hub) handleOpenDM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	peer := strings.TrimSpace(strings.ToLower(body.To))
	if peer == "" || peer == user || !validName(peer) {
		http.Error(w, "invalid peer", http.StatusBadRequest)
		return
	}
	exists, err := h.redis.Exists(r.Context(), "user:"+peer).Result()
	if err != nil || exists == 0 {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if h.blockedEither(r.Context(), user, peer) {
		http.Error(w, "blocked", http.StatusForbidden)
		return
	}
	a, b := user, peer
	if a > b {
		a, b = b, a
	}
	room := "dm:" + a + ":" + b
	_ = h.redis.SAdd(r.Context(), roomsSet, room).Err()
	_ = h.redis.SAdd(r.Context(), "user:"+user+":dms", room).Err()
	_ = h.redis.SAdd(r.Context(), "user:"+peer+":dms", room).Err()
	writeJSON(w, map[string]string{"name": room, "kind": "dm", "peer": peer})
}

func (h *Hub) handleRoomSubroutes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.userFromRequest(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	room := strings.ToLower(parts[0])
	// dm rooms contain colons: dm:a:b/messages — path splits wrong.
	// Support both /api/rooms/{room}/messages and encode room in query.
	if strings.HasPrefix(parts[0], "dm") && len(parts) >= 3 {
		// Path like dm / alice / bob / messages — unlikely with EncodeURIComponent
	}
	switch parts[len(parts)-1] {
	case "messages":
		room = strings.TrimSuffix(strings.Trim(path, "/"), "/messages")
		room = strings.ToLower(room)
		h.handleMessages(w, r, room)
	case "presence":
		room = strings.TrimSuffix(strings.Trim(path, "/"), "/presence")
		room = strings.ToLower(room)
		h.handlePresence(w, r, room)
	case "reads":
		room = strings.TrimSuffix(strings.Trim(path, "/"), "/reads")
		room = strings.ToLower(room)
		h.handleReads(w, r, room)
	case "pins":
		room = strings.TrimSuffix(strings.Trim(path, "/"), "/pins")
		room = strings.ToLower(room)
		h.handlePin(w, r, room)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *Hub) handleMessages(w http.ResponseWriter, r *http.Request, room string) {
	user, _ := h.userFromRequest(r)
	if !h.canJoinRoom(r.Context(), user, room) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	h.clearUnread(r.Context(), user, room)
	items, err := h.redis.LRange(r.Context(), "history:"+room, 0, 49).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().UnixMilli()
	msgs := make([]json.RawMessage, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		var m map[string]interface{}
		if json.Unmarshal([]byte(items[i]), &m) == nil {
			if str(m["ephemeral"]) == "24h" {
				if exp := numI64(m["expire_at"]); exp > 0 && now > exp {
					continue
				}
			}
		}
		msgs = append(msgs, json.RawMessage(items[i]))
	}
	writeJSON(w, msgs)
}

func numI64(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case json.Number:
		n, _ := t.Int64()
		return n
	default:
		return 0
	}
}

func (h *Hub) handlePresence(w http.ResponseWriter, r *http.Request, room string) {
	user, _ := h.userFromRequest(r)
	if !h.canJoinRoom(r.Context(), user, room) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	members, err := h.redis.SMembers(r.Context(), "presence:"+room).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"room": room, "online": members, "count": len(members)})
}

func (h *Hub) handleReads(w http.ResponseWriter, r *http.Request, room string) {
	user, _ := h.userFromRequest(r)
	if !h.canJoinRoom(r.Context(), user, room) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	m, err := h.redis.HGetAll(r.Context(), "read:"+room).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, m)
}

func (h *Hub) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := h.userFromRequest(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5 MB
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Error(w, "file too large (max 5MB)", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
		".pdf": true, ".txt": true, ".zip": true,
	}
	if !allowed[ext] {
		http.Error(w, "unsupported file type", http.StatusBadRequest)
		return
	}

	id := make([]byte, 16)
	_, _ = rand.Read(id)
	name := hex.EncodeToString(id) + ext
	if err := os.MkdirAll(h.uploads, 0o755); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	dstPath := filepath.Join(h.uploads, name)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "write failed", http.StatusInternalServerError)
		return
	}

	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	kind := "file"
	if strings.HasPrefix(mime, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" {
		kind = "image"
	}
	meta, _ := json.Marshal(map[string]string{
		"name": header.Filename,
		"mime": mime,
	})
	writeJSON(w, map[string]string{
		"url":  "/uploads/" + name,
		"type": kind,
		"meta": string(meta),
		"name": header.Filename,
	})
}

func (h *Hub) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := h.redis.Ping(r.Context()).Err(); err != nil {
		http.Error(w, `{"ok":false}`, http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "service": "gateway"})
}
