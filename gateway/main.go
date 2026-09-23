package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	HTTPAddr  string
	RedisAddr string
	Uploads   string
}

func loadConfig() Config {
	uploads := envOr("UPLOAD_DIR", filepath.Join("..", "uploads"))
	return Config{
		HTTPAddr:  envOr("HTTP_ADDR", ":8080"),
		RedisAddr: envOr("REDIS_ADDR", "127.0.0.1:6379"),
		Uploads:   uploads,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func seedRooms(ctx context.Context, rdb *redis.Client) {
	for _, name := range []string{"general", "random", "dev"} {
		_ = rdb.SAdd(ctx, roomsSet, name).Err()
	}
}

func main() {
	cfg := loadConfig()
	_ = os.MkdirAll(cfg.Uploads, 0o755)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}
	seedRooms(ctx, rdb)
	seedDemoUsers(ctx, rdb)
	seedDemoChats(ctx, rdb)
	seedDemoPresence(ctx, rdb)
	go refreshDemoPresenceLoop(ctx, rdb)

	hub := NewHub(rdb, cfg.Uploads)
	go hub.subscribeRedis(ctx)
	hub.startFeatureLoops(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", hub.handleHealth)
	mux.HandleFunc("/api/auth/register", hub.handleRegister)
	mux.HandleFunc("/api/auth/login", hub.handleLogin)
	mux.HandleFunc("/api/me", hub.handleMe)
	mux.HandleFunc("/api/users", hub.handleUsers)
	mux.HandleFunc("/api/users/", hub.handleUserRoutes)
	mux.HandleFunc("/api/dm", hub.handleOpenDM)
	mux.HandleFunc("/api/upload", hub.handleUpload)
	mux.HandleFunc("/api/focus", hub.handleFocus)
	mux.HandleFunc("/api/unread", hub.handleUnread)
	mux.HandleFunc("/api/search", hub.handleSearch)
	mux.HandleFunc("/api/invite", hub.handleInvite)
	mux.HandleFunc("/api/signal", hub.handleSignal)
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
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.Uploads))))
	mux.Handle("/", http.FileServer(http.Dir("../web")))

	log.Printf("gateway listening on %s (redis %s, uploads %s)", cfg.HTTPAddr, cfg.RedisAddr, cfg.Uploads)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, mux))
}
