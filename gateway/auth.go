package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	usersSet     = "chat:users"
	sessionTTL   = 7 * 24 * time.Hour
	bcryptCost   = 10
	minPassword  = 4
)

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

func (h *Hub) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	user := strings.TrimSpace(strings.ToLower(req.Username))
	pass := req.Password
	if !validName(user) || len(user) < 2 || len(user) > 24 {
		http.Error(w, "invalid username", http.StatusBadRequest)
		return
	}
	if len(pass) < minPassword || len(pass) > 72 {
		http.Error(w, "password must be 4-72 chars", http.StatusBadRequest)
		return
	}

	exists, err := h.redis.Exists(r.Context(), "user:"+user).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists > 0 {
		http.Error(w, "username taken", http.StatusConflict)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcryptCost)
	if err != nil {
		http.Error(w, "hash failed", http.StatusInternalServerError)
		return
	}
	pipe := h.redis.TxPipeline()
	pipe.HSet(r.Context(), "user:"+user, map[string]interface{}{
		"password":  string(hash),
		"createdAt": time.Now().Unix(),
	})
	pipe.SAdd(r.Context(), usersSet, user)
	if _, err := pipe.Exec(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	token, err := h.createSession(r.Context(), user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, authResponse{Token: token, Username: user})
}

func (h *Hub) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	user := strings.TrimSpace(strings.ToLower(req.Username))
	stored, err := h.redis.HGet(r.Context(), "user:"+user, "password").Result()
	if err == redis.Nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(stored), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := h.createSession(r.Context(), user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, authResponse{Token: token, Username: user})
}

func (h *Hub) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]string{"username": user})
}

func (h *Hub) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := h.userFromRequest(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	users, err := h.redis.SMembers(r.Context(), usersSet).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type userInfo struct {
		Username string `json:"username"`
		Status   string `json:"status"`
		Room     string `json:"room,omitempty"`
	}
	out := make([]userInfo, 0, len(users))
	for _, u := range users {
		focus, _ := h.redis.HGetAll(r.Context(), "focus:"+u).Result()
		st := focus["status"]
		if st == "" {
			st = "away"
		}
		out = append(out, userInfo{Username: u, Status: st, Room: focus["room"]})
	}
	writeJSON(w, out)
}

func (h *Hub) createSession(ctx context.Context, user string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	if err := h.redis.Set(ctx, "session:"+token, user, sessionTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func (h *Hub) userFromToken(ctx context.Context, token string) (string, bool) {
	if token == "" {
		return "", false
	}
	user, err := h.redis.Get(ctx, "session:"+token).Result()
	if err != nil {
		return "", false
	}
	_ = h.redis.Expire(ctx, "session:"+token, sessionTTL).Err()
	return user, true
}

func (h *Hub) userFromRequest(r *http.Request) (string, bool) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	return h.userFromToken(r.Context(), token)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
